package transactions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/receivevault"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func ResolveBridgeVaultAddress(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceChainID int,
	destChainID int,
	vaultAddress string,
) (string, error) {
	if !receivevault.IsReceiveVaultSupported(destChainID) {
		return "", fmt.Errorf("Deposit addresses are not supported on chain %d", destChainID)
	}
	if vaultAddress == "" {
		sourceNetwork := receivevault.ReceiveVaultNetworkOf(sourceChainID)
		destNetwork := receivevault.ReceiveVaultNetworkOf(destChainID)
		if sourceNetwork != destNetwork {
			return "", fmt.Errorf("vaultAddress is required to bridge from %s to %s", sourceNetwork, destNetwork)
		}
		record, err := hinkal.CreateReceiveAddress(ctx, destChainID)
		if err != nil {
			return "", err
		}
		if !receivevault.IsReceiveVaultExpired(receivevault.TermsFromReceiveVaultRecord(record), constants.BridgeVaultMinTTLSeconds) {
			return record.VaultAddress, nil
		}
		fresh, err := hinkal.CreateReceiveAddress(ctx, destChainID, true)
		if err != nil {
			return "", err
		}
		return fresh.VaultAddress, nil
	}

	normalizedVaultAddress, err := toBase58IfTron(destChainID, vaultAddress)
	if err != nil {
		return "", err
	}
	isReceiveVault, err := api.CheckReceiveVaultAddress(ctx, normalizedVaultAddress)
	if err != nil {
		return "", err
	}
	if !isReceiveVault {
		return "", fmt.Errorf("%s is not a Hinkal deposit address", normalizedVaultAddress)
	}
	return normalizedVaultAddress, nil
}

func hinkalLifiBridgePrivateOut(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	vaultAddress string,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (bridging.PrivateBridgeResult, error) {
	chainID := sourceToken.ChainID
	tokenAddress := sourceToken.Erc20TokenAddress

	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	nonce, err := utils.RandomBigInt(6)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	sourceSubAccount, err := privatewallet.BuildTempSubAccountForChain(hinkal.GetUserKeys(), chainID, nonce, false)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	sourceAddress, err := privatewallet.AddressFromSubAccount(chainID, sourceSubAccount)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	var quote types.BridgeQuote
	var ethereumAddress string
	var group errgroup.Group
	group.Go(func() error {
		resolved, err := web3.GetLifiPrice(ctx, sourceToken, destToken, amount, slippage, sourceAddress, vaultAddress)
		quote = resolved
		return err
	})
	group.Go(func() error {
		resolved, err := hinkal.GetEthereumAddress(ctx)
		ethereumAddress = resolved
		return err
	})
	if err := group.Wait(); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	if _, err := api.AddTemporaryWalletNonce(
		ctx,
		chainID,
		utils.HashEthereumAddress(ethereumAddress),
		int(nonce.Int64()),
		types.TemporaryWalletRecoveryDestinationConfidential,
	); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	nativeFee := quote.NativeFee
	if nativeFee == nil {
		nativeFee = big.NewInt(0)
	}
	isNativeInput := strings.EqualFold(tokenAddress, constants.ZeroAddress)
	needsNativeFee := nativeFee.Sign() > 0 && !isNativeInput
	bridgeFundingAmount := new(big.Int).Set(bridgeAmount)
	if isNativeInput {
		bridgeFundingAmount = new(big.Int).Add(bridgeAmount, nativeFee)
	}

	erc20Tokens := []types.ERC20Token{sourceToken}
	deltaChanges := []*big.Int{new(big.Int).Neg(bridgeFundingAmount)}
	if needsNativeFee {
		nativeToken, err := web3.GetErc20TokenFromAPI(ctx, chainID, constants.ZeroAddress)
		if err != nil {
			return bridging.PrivateBridgeResult{}, err
		}
		if nativeToken == nil {
			return bridging.PrivateBridgeResult{}, errors.New("transactions: native token not found for bridge fee")
		}
		erc20Tokens = append(erc20Tokens, *nativeToken)
		deltaChanges = append(deltaChanges, new(big.Int).Neg(nativeFee))
	}

	ops, err := privatewallet.CreateLifiBridgeOps(chainID, sourceAddress, lifiRouterAddress, tokenAddress, bridgeFundingAmount, bridgeAmount, quote)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	emporiumTokenChanges := make([]bridging.TokenChange, len(erc20Tokens))
	for i, token := range erc20Tokens {
		emporiumTokenChanges[i] = bridging.TokenChange{Token: token, Amount: deltaChanges[i]}
	}
	onChainCreation := make([]bool, len(deltaChanges))

	resolvedFeeToken := feeToken
	if resolvedFeeToken == "" {
		resolvedFeeToken = tokenAddress
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = normalizeFeeStructure(*feeStructureOverride)
	} else {
		erc20Addresses := make([]string, len(erc20Tokens))
		for i, token := range erc20Tokens {
			erc20Addresses[i] = token.Erc20TokenAddress
		}
		calls := make([]types.CallInfo, len(ops))
		for i, op := range ops {
			call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, sourceAddress, chainID)
			if err != nil {
				return bridging.PrivateBridgeResult{}, err
			}
			calls[i] = call
		}
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, resolvedFeeToken, erc20Addresses, types.ExternalActionEmporium, calls, nil, nil)
		if err != nil {
			return bridging.PrivateBridgeResult{}, err
		}
	}
	feeStructure.VariableRate = constants.HinkalPrivateSendVariableRate()
	bridgeFeeStructure := fees.CalculateModifiedFeeStructure(ctx, chainID, sourceToken, bridgeAmount, feeStructure)

	sourceTxHash, err := actionPrivateWallet(
		ctx,
		hinkal,
		chainID,
		erc20Tokens,
		deltaChanges,
		onChainCreation,
		ops,
		emporiumTokenChanges,
		&sourceSubAccount,
		resolvedFeeToken,
		&bridgeFeeStructure,
		"",
		action,
		nil,
	)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	return bridging.PrivateBridgeResult{
		SourceTxHash:           sourceTxHash,
		DestinationTokenAmount: quote.ExpectedAmount.String(),
	}, nil
}

func hinkalNearBridgePrivateOut(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	vaultAddress string,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (bridging.PrivateBridgeResult, error) {
	var originAsset, destinationAsset string
	var group errgroup.Group
	group.Go(func() error {
		resolved, err := integrations.ResolveNearAssetID(ctx, sourceToken.ChainID, sourceToken.Erc20TokenAddress)
		originAsset = resolved
		return err
	})
	group.Go(func() error {
		resolved, err := integrations.ResolveNearAssetID(ctx, destToken.ChainID, destToken.Erc20TokenAddress)
		destinationAsset = resolved
		return err
	})
	if err := group.Wait(); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	slippageBps := int(math.Round(slippage * 10_000))
	if slippageBps == 0 {
		slippageBps = constants.NearBridgeSlippageBPS
	}
	refundTo, err := hinkal.GetUserKeys().GetNearIntentsAccountID()
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	quoteResp, err := api.GetNearIntentsQuote(ctx, types.NearIntentsQuoteRequest{
		Dry:               false,
		SwapType:          "EXACT_INPUT",
		SlippageTolerance: slippageBps,
		OriginAsset:       originAsset,
		DepositType:       "ORIGIN_CHAIN",
		DestinationAsset:  destinationAsset,
		Amount:            bridgeAmount.String(),
		Recipient:         vaultAddress,
		RecipientType:     "DESTINATION_CHAIN",
		RefundTo:          refundTo,
		RefundType:        "INTENTS",
		Deadline:          nearBridgeDeadline(time.Now().UnixMilli()),
	})
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	if quoteResp.Quote.DepositAddress == "" {
		return bridging.PrivateBridgeResult{}, errors.New("NEAR Intents quote returned no deposit address")
	}

	resolvedFeeToken := feeToken
	if resolvedFeeToken == "" {
		resolvedFeeToken = sourceToken.Erc20TokenAddress
	}
	var sourceTxHash string
	if constants.IsSolanaLike(sourceToken.ChainID) {
		sourceTxHash, err = HinkalSolanaWithdraw(
			ctx,
			hinkal,
			[]types.ERC20Token{sourceToken},
			[]*big.Int{new(big.Int).Neg(bridgeAmount)},
			quoteResp.Quote.DepositAddress,
			feeStructureOverride,
			constants.HinkalPrivateSendVariableRate(),
			action,
		)
	} else {
		_, sourceTxHash, err = HinkalWithdraw(
			ctx,
			hinkal,
			[]types.ERC20Token{sourceToken},
			[]*big.Int{new(big.Int).Neg(bridgeAmount)},
			quoteResp.Quote.DepositAddress,
			false,
			resolvedFeeToken,
			feeStructureOverride,
			constants.HinkalPrivateSendVariableRate(),
			action,
		)
	}
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	return bridging.PrivateBridgeResult{
		SourceTxHash:           sourceTxHash,
		DestinationTokenAmount: quoteResp.Quote.AmountOut,
	}, nil
}

func HinkalBridgePrivateToPrivate(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	vaultAddress string,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (bridging.PrivateBridgeResult, error) {
	if slippage == 0 {
		slippage = constants.DefaultBridgingSlippageDecimal
	}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, sourceToken.ChainID); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	var destinationVaultAddress string
	var settleFeeStructure types.FeeStructure
	var group errgroup.Group
	group.Go(func() error {
		resolved, err := ResolveBridgeVaultAddress(ctx, hinkal, sourceToken.ChainID, destToken.ChainID, vaultAddress)
		destinationVaultAddress = resolved
		return err
	})
	group.Go(func() error {
		resolved, err := receivevault.GetReceiveVaultSettleFeeStructure(ctx, destToken.ChainID, destToken.Erc20TokenAddress)
		settleFeeStructure = resolved
		return err
	})
	if err := group.Wait(); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	bridgePrivateOut := hinkalLifiBridgePrivateOut
	if constants.IsNearIntentsBridgeSource(sourceToken.ChainID) {
		bridgePrivateOut = hinkalNearBridgePrivateOut
	}
	bridgeOut, err := bridgePrivateOut(
		ctx,
		hinkal,
		sourceToken,
		destToken,
		amount,
		destinationVaultAddress,
		slippage,
		feeToken,
		feeStructureOverride,
		action,
	)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	landedAmount := big.NewInt(0)
	if bridgeOut.DestinationTokenAmount != "" {
		if _, ok := landedAmount.SetString(bridgeOut.DestinationTokenAmount, 10); !ok {
			return bridging.PrivateBridgeResult{}, fmt.Errorf("transactions: invalid bridge destination amount %q", bridgeOut.DestinationTokenAmount)
		}
	}
	shieldedAmount := new(big.Int).Sub(landedAmount, fees.CalculateTotalFee(landedAmount, settleFeeStructure))
	if shieldedAmount.Sign() < 0 {
		shieldedAmount = big.NewInt(0)
	}

	return bridging.PrivateBridgeResult{
		SourceTxHash:           bridgeOut.SourceTxHash,
		DestinationTokenAmount: shieldedAmount.String(),
	}, nil
}
