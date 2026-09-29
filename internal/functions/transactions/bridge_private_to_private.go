package transactions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

var errBridgeArrivalTimeout = errors.New("transactions: bridge did not land on destination temp wallet within timeout")

type bridgeClaimableRecipient struct {
	Signature  string
	EthAddress string
	Keys       *cryptokeys.UserKeys
}

type bridgeResolvedRecipient struct {
	RecipientInfo string
	Claimable     *bridgeClaimableRecipient
}

func resolveBridgeRecipient(hinkal ihinkal.HinkalInternal, recipient bridging.PrivateBridgeRecipient) (bridgeResolvedRecipient, error) {
	if !recipient.IsClaimable() {
		return bridgeResolvedRecipient{RecipientInfo: recipient.RecipientInfo}, nil
	}

	signature, err := hinkal.GetUserKeys().GetClaimableSignatureFromNonce(recipient.ClaimableNonce)
	if err != nil {
		return bridgeResolvedRecipient{}, err
	}
	keys := cryptokeys.NewUserKeys(signature)
	recipientInfo, err := pretransaction.GetRecipientInfoFromUserKeys(keys)
	if err != nil {
		return bridgeResolvedRecipient{}, err
	}

	return bridgeResolvedRecipient{
		RecipientInfo: recipientInfo,
		Claimable: &bridgeClaimableRecipient{
			Signature:  signature,
			EthAddress: recipient.RecipientEthAddress,
			Keys:       keys,
		},
	}, nil
}

func registerTemporaryWalletNonces(ctx context.Context, hinkal ihinkal.HinkalInternal, nonce *big.Int, chainIDs ...int) error {
	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(ethereumAddress)

	seen := make(map[int]struct{}, len(chainIDs))
	for _, chainID := range chainIDs {
		if _, ok := seen[chainID]; ok {
			continue
		}
		seen[chainID] = struct{}{}
		if _, err := api.AddTemporaryWalletNonce(ctx, chainID, hashedEthereumAddress, int(nonce.Int64()), types.TemporaryWalletRecoveryDestinationConfidential); err != nil {
			return err
		}
	}
	return nil
}

func hinkalLifiBridgePrivateOut(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (bridging.PrivateBridgeOutResult, error) {
	chainID := sourceToken.ChainID
	destChainID := destToken.ChainID
	tokenAddress := sourceToken.Erc20TokenAddress

	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	nonce, err := utils.RandomBigInt(6)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	tempWallets, err := privatewallet.DeriveBridgeTempWallets(hinkal.GetUserKeys(), chainID, destChainID, nonce)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	sourceAddress, err := privatewallet.AddressFromSubAccount(chainID, tempWallets.SourceSubAccount)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	destinationAddress, err := privatewallet.AddressFromSubAccount(destChainID, tempWallets.DestinationSubAccount)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	quote, err := web3.GetLifiPrice(ctx, sourceToken, destToken, amount, slippage, sourceAddress, destinationAddress)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	if err := registerTemporaryWalletNonces(ctx, hinkal, nonce, chainID, destChainID); err != nil {
		return bridging.PrivateBridgeOutResult{}, err
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
			return bridging.PrivateBridgeOutResult{}, err
		}
		if nativeToken == nil {
			return bridging.PrivateBridgeOutResult{}, errors.New("transactions: native token not found for bridge fee")
		}
		erc20Tokens = append(erc20Tokens, *nativeToken)
		deltaChanges = append(deltaChanges, new(big.Int).Neg(nativeFee))
	}

	ops, err := privatewallet.CreateLifiBridgeOps(chainID, sourceAddress, lifiRouterAddress, tokenAddress, bridgeFundingAmount, bridgeAmount, quote)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
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

	sourceSubAccount := tempWallets.SourceSubAccount
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
		feeStructureOverride,
		"",
		action,
		nil,
	)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	return bridging.PrivateBridgeOutResult{
		TempSubAccount: tempWallets.DestinationSubAccount,
		SourceTxHash:   sourceTxHash,
		ExpectedAmount: quote.ExpectedAmount,
	}, nil
}

func hinkalNearBridgePrivateOut(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (bridging.PrivateBridgeOutResult, error) {
	destChainID := destToken.ChainID

	nonce, err := utils.RandomBigInt(6)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	tempSubAccount, err := privatewallet.BuildTempSubAccountForChain(hinkal.GetUserKeys(), destChainID, nonce, false)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	tempWalletAddress, err := privatewallet.AddressFromSubAccount(destChainID, tempSubAccount)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	if err := registerTemporaryWalletNonces(ctx, hinkal, nonce, destChainID); err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	originAsset, err := integrations.ResolveNearAssetID(ctx, sourceToken.ChainID, sourceToken.Erc20TokenAddress)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	destinationAsset, err := integrations.ResolveNearAssetID(ctx, destChainID, destToken.Erc20TokenAddress)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	slippageBps := int(math.Round(slippage * 10_000))
	if slippageBps == 0 {
		slippageBps = constants.NearBridgeSlippageBPS
	}
	refundTo, err := hinkal.GetUserKeys().GetNearIntentsAccountID()
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	quoteResp, err := api.GetNearIntentsQuote(ctx, types.NearIntentsQuoteRequest{
		Dry:               false,
		SwapType:          "EXACT_INPUT",
		SlippageTolerance: slippageBps,
		OriginAsset:       originAsset,
		DepositType:       "ORIGIN_CHAIN",
		DestinationAsset:  destinationAsset,
		Amount:            bridgeAmount.String(),
		Recipient:         tempWalletAddress,
		RecipientType:     "DESTINATION_CHAIN",
		RefundTo:          refundTo,
		RefundType:        "INTENTS",
		Deadline:          nearBridgeDeadline(time.Now().UnixMilli()),
	})
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}
	if quoteResp.Quote.DepositAddress == "" {
		return bridging.PrivateBridgeOutResult{}, errors.New("NEAR Intents quote returned no deposit address")
	}

	resolvedFeeToken := feeToken
	if resolvedFeeToken == "" {
		resolvedFeeToken = sourceToken.Erc20TokenAddress
	}
	_, sourceTxHash, err := hinkal.Withdraw(
		ctx,
		sourceToken.ChainID,
		[]string{sourceToken.Erc20TokenAddress},
		[]*big.Int{new(big.Int).Neg(bridgeAmount)},
		quoteResp.Quote.DepositAddress,
		false,
		resolvedFeeToken,
		feeStructureOverride,
		action,
	)
	if err != nil {
		return bridging.PrivateBridgeOutResult{}, err
	}

	expectedAmount, ok := new(big.Int).SetString(quoteResp.Quote.AmountOut, 10)
	if !ok {
		return bridging.PrivateBridgeOutResult{}, fmt.Errorf("transactions: invalid NEAR Intents quote amountOut %q", quoteResp.Quote.AmountOut)
	}

	return bridging.PrivateBridgeOutResult{
		TempSubAccount: tempSubAccount,
		SourceTxHash:   sourceTxHash,
		ExpectedAmount: expectedAmount,
	}, nil
}

func waitForBridgeArrival(
	ctx context.Context,
	destChainID int,
	tempWalletAddress string,
	destToken types.ERC20Token,
	expectedAmount *big.Int,
	slippage float64,
) (*big.Int, error) {
	slippageScaled := big.NewInt(int64(math.Round(slippage * float64(constants.SlippageScalingFactor))))
	minAmount := new(big.Int).Sub(expectedAmount, new(big.Int).Quo(new(big.Int).Mul(expectedAmount, slippageScaled), big.NewInt(constants.SlippageScalingFactor)))
	floor := big.NewInt(1)
	if minAmount.Sign() > 0 {
		floor = minAmount
	}

	deadline := time.Now().Add(constants.BridgeArrivalTimeout)
	for time.Now().Before(deadline) {
		balance, err := web3.GetPublicBalanceByAddress(ctx, destChainID, destToken, tempWalletAddress)
		if err == nil && balance != nil && balance.Cmp(floor) >= 0 {
			return balance, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(constants.BridgeArrivalPollInterval):
		}
	}
	return nil, fmt.Errorf("%w: wallet=%s chain=%d", errBridgeArrivalTimeout, tempWalletAddress, destChainID)
}

func hinkalShieldBridgedFunds(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	destToken types.ERC20Token,
	tempSubAccount types.TemporarySubAccount,
	landedAmount *big.Int,
	recipient bridging.PrivateBridgeRecipient,
) (string, *big.Int, error) {
	destChainID := destToken.ChainID
	destTokenAddress := destToken.Erc20TokenAddress
	tempWalletAddress, err := privatewallet.AddressFromSubAccount(destChainID, tempSubAccount)
	if err != nil {
		return "", nil, err
	}

	resolved, err := resolveBridgeRecipient(hinkal, recipient)
	if err != nil {
		return "", nil, err
	}

	// Persist the claimable key before any shield transaction can be submitted.
	if resolved.Claimable != nil {
		senderAddress, err := hinkal.GetEthereumAddressByChain(ctx, destChainID)
		if err != nil {
			return "", nil, err
		}
		shieldedPrivateKey, err := resolved.Claimable.Keys.GetShieldedPrivateKey()
		if err != nil {
			return "", nil, err
		}
		senderSignature, err := hinkal.GetUserKeys().GetSignature()
		if err != nil {
			return "", nil, err
		}
		if err := enclave.StoreClaimableKeyInEnclave(
			ctx,
			senderAddress,
			resolved.Claimable.EthAddress,
			shieldedPrivateKey,
			destChainID,
			resolved.Claimable.Signature,
			senderSignature,
		); err != nil {
			return "", nil, err
		}
	}

	if constants.IsSolanaLike(destChainID) {
		return HinkalSolanaSponsoredShield(ctx, hinkal, destToken, tempSubAccount, landedAmount, resolved.RecipientInfo, "", constants.HinkalShieldVariableRate())
	}

	transferOps, err := privatewallet.CreateTransferEmporiumOpsBatch(destChainID, []string{destTokenAddress}, []*big.Int{landedAmount}, "")
	if err != nil {
		return "", nil, err
	}
	calls := make([]types.CallInfo, len(transferOps))
	for i, op := range transferOps {
		call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, tempWalletAddress, destChainID)
		if err != nil {
			return "", nil, err
		}
		calls[i] = call
	}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, destChainID, destTokenAddress, []string{destTokenAddress}, types.ExternalActionEmporium, calls, constants.HinkalPrivateSendVariableRate(), nil)
	if err != nil {
		return "", nil, err
	}
	modifiedFeeStructure := fees.CalculateModifiedFeeStructure(ctx, destChainID, destToken, landedAmount, feeStructure)

	shieldedAmount := new(big.Int).Sub(landedAmount, fees.CalculateTotalFee(landedAmount, modifiedFeeStructure))
	if shieldedAmount.Sign() <= 0 {
		return "", nil, errors.New(errorhandling.ErrCodeInsufficientFundsToTransact)
	}

	destTxHash, err := HinkalProxyToPrivate(ctx, hinkal, destChainID, destToken, shieldedAmount, tempSubAccount, resolved.RecipientInfo, modifiedFeeStructure.FeeToken, &modifiedFeeStructure, "")
	if err != nil {
		return "", nil, err
	}

	return destTxHash, shieldedAmount, nil
}

func HinkalBridgePrivateToPrivate(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	recipient bridging.PrivateBridgeRecipient,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (bridging.PrivateBridgeResult, error) {
	if slippage == 0 {
		slippage = constants.DefaultBridgingSlippageDecimal
	}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, sourceToken.ChainID, destToken.ChainID); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	var bridgeOut bridging.PrivateBridgeOutResult
	var err error
	if constants.IsNearIntentsBridgeSource(sourceToken.ChainID) {
		bridgeOut, err = hinkalNearBridgePrivateOut(ctx, hinkal, sourceToken, destToken, amount, slippage, feeToken, feeStructureOverride, action)
	} else {
		bridgeOut, err = hinkalLifiBridgePrivateOut(ctx, hinkal, sourceToken, destToken, amount, slippage, feeToken, feeStructureOverride, action)
	}
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	destChainID := destToken.ChainID
	tempWalletAddress, err := privatewallet.AddressFromSubAccount(destChainID, bridgeOut.TempSubAccount)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	bridgedAmount, err := waitForBridgeArrival(ctx, destChainID, tempWalletAddress, destToken, bridgeOut.ExpectedAmount, slippage)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, destChainID); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	destTxHash, shieldedAmount, err := hinkalShieldBridgedFunds(ctx, hinkal, destToken, bridgeOut.TempSubAccount, bridgedAmount, recipient)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	return bridging.PrivateBridgeResult{
		SourceTxHash:           bridgeOut.SourceTxHash,
		DestTxHash:             destTxHash,
		DestinationTokenAmount: shieldedAmount.String(),
	}, nil
}
