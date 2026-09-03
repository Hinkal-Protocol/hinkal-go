package transactions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/onchainutxos"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

var errBridgeArrivalTimeout = errors.New("transactions: bridge did not land on destination temp wallet within timeout")

func deriveBridgeTempWallet(hinkal ihinkal.HinkalInternal, chainID int, nonce *big.Int) (types.TemporarySubAccount, error) {
	privateKey, err := hinkal.GetUserKeys().GetSignerPrivateKeyFromNonce(nonce)
	if err != nil {
		return types.TemporarySubAccount{}, err
	}
	ethAddress, err := privatewallet.SignerAddressFromPrivateKey(chainID, privateKey)
	if err != nil {
		return types.TemporarySubAccount{}, err
	}
	return types.TemporarySubAccount{Index: int(nonce.Int64()), EthAddress: ethAddress, PrivateKey: privateKey}, nil
}

func hinkalBridgePrivateOut(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	slippage float64,
	feeToken string,
) (tempAccount types.TemporarySubAccount, sourceTxHash string, quote types.BridgeQuote, err error) {
	chainID := sourceToken.ChainID
	destChainID := destToken.ChainID
	tokenAddress := sourceToken.Erc20TokenAddress

	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}

	nonce, err := utils.RandomBigInt(6)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}
	tempAccount, err = deriveBridgeTempWallet(hinkal, chainID, nonce)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}

	quote, err = web3.GetLifiPrice(ctx, sourceToken, destToken, amount, slippage, tempAccount.EthAddress, tempAccount.EthAddress)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}

	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}

	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(ethereumAddress)
	if _, err := api.AddTemporaryWalletNonce(ctx, chainID, hashedEthereumAddress, tempAccount.Index, types.TemporaryWalletRecoveryDestinationConfidential); err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
	}
	if _, err := api.AddTemporaryWalletNonce(ctx, destChainID, hashedEthereumAddress, tempAccount.Index, types.TemporaryWalletRecoveryDestinationConfidential); err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
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
			return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
		}
		if nativeToken == nil {
			return types.TemporarySubAccount{}, "", types.BridgeQuote{}, errors.New("transactions: native token not found for bridge fee")
		}
		erc20Tokens = append(erc20Tokens, *nativeToken)
		deltaChanges = append(deltaChanges, new(big.Int).Neg(nativeFee))
	}

	ops, err := privatewallet.CreateLifiBridgeOps(chainID, tempAccount.EthAddress, lifiRouterAddress, tokenAddress, bridgeFundingAmount, bridgeAmount, quote)
	if err != nil {
		return types.TemporarySubAccount{}, "", types.BridgeQuote{}, err
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

	sourceTxHash, err = actionPrivateWallet(
		ctx,
		hinkal,
		chainID,
		erc20Tokens,
		deltaChanges,
		onChainCreation,
		ops,
		emporiumTokenChanges,
		&tempAccount,
		resolvedFeeToken,
		nil,
		"",
		types.AdminPrivateToPrivateSend,
		nil,
	)
	return tempAccount, sourceTxHash, quote, err
}

func waitForBridgeArrival(ctx context.Context, destChainID int, tempWalletAddress, destTokenAddress string, quote types.BridgeQuote, slippage float64) (*big.Int, error) {
	slippageScaled := big.NewInt(int64(math.Round(slippage * float64(constants.SlippageScalingFactor))))
	minAmount := new(big.Int).Sub(quote.ExpectedAmount, new(big.Int).Quo(new(big.Int).Mul(quote.ExpectedAmount, slippageScaled), big.NewInt(constants.SlippageScalingFactor)))
	floor := big.NewInt(1)
	if minAmount.Sign() > 0 {
		floor = minAmount
	}

	deadline := time.Now().Add(constants.BridgeArrivalTimeout)
	for time.Now().Before(deadline) {
		balance, err := web3.GetPublicBalanceByAddress(ctx, destChainID, destTokenAddress, tempWalletAddress)
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
	tempAccount types.TemporarySubAccount,
	landedAmount *big.Int,
	recipient bridging.PrivateBridgeRecipient,
) (string, *bridging.PendingEnclaveUtxo, error) {
	destChainID := destToken.ChainID
	destTokenAddress := destToken.Erc20TokenAddress

	transferOps, err := privatewallet.CreateTransferEmporiumOpsBatch(destChainID, []string{destTokenAddress}, []*big.Int{landedAmount}, "")
	if err != nil {
		return "", nil, err
	}
	calls := make([]types.CallInfo, len(transferOps))
	for i, op := range transferOps {
		call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, tempAccount.EthAddress, destChainID)
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

	var recipientInfo string
	var claimableKeys *cryptokeys.UserKeys
	var claimableSignature string
	if recipient.IsClaimable() {
		signature, err := hinkal.GetUserKeys().GetClaimableSignatureFromNonce(recipient.ClaimableNonce)
		if err != nil {
			return "", nil, err
		}
		claimableSignature = signature
		claimableKeys = cryptokeys.NewUserKeys(signature)
		recipientInfo, err = pretransaction.GetRecipientInfoFromUserKeys(claimableKeys)
		if err != nil {
			return "", nil, err
		}
	} else {
		recipientInfo = recipient.RecipientInfo
	}

	destTxHash, err := hinkalProxyToPrivate(ctx, hinkal, destChainID, destToken, shieldedAmount, tempAccount, recipientInfo, modifiedFeeStructure.FeeToken, &modifiedFeeStructure, types.AdminProxyToPrivateSend)
	if err != nil {
		return "", nil, err
	}

	if claimableKeys == nil {
		return destTxHash, nil, nil
	}

	if _, err := hinkal.WaitForTransaction(ctx, destChainID, destTxHash, 1); err != nil {
		return "", nil, fmt.Errorf("transactions: could not confirm bridge shield tx %s: %w", destTxHash, err)
	}
	client, err := hinkal.GetFetchClient(destChainID)
	if err != nil {
		return "", nil, err
	}
	receipt, err := web3.FetchTransactionReceiptWithRetry(ctx, client, destTxHash)
	if err != nil {
		return "", nil, err
	}
	utxos, err := onchainutxos.DecodeFromReceipt(receipt, claimableKeys, destChainID, destTokenAddress)
	if err != nil {
		return "", nil, err
	}
	if len(utxos) == 0 {
		return "", nil, errors.New("transactions: could not find recipient utxo in bridge shield tx")
	}

	senderAddress, err := hinkal.GetEthereumAddressByChain(ctx, destChainID)
	if err != nil {
		return "", nil, err
	}
	return destTxHash, &bridging.PendingEnclaveUtxo{
		ChainID:            destChainID,
		SenderAddress:      senderAddress,
		RecipientAddress:   recipient.RecipientEthAddress,
		ClaimableSignature: claimableSignature,
		Utxo:               utxos[0],
	}, nil
}

func HinkalBridgePrivateToPrivate(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken, destToken types.ERC20Token,
	amount string,
	recipient bridging.PrivateBridgeRecipient,
	slippage float64,
	feeToken string,
) (bridging.PrivateBridgeResult, error) {
	if slippage == 0 {
		slippage = constants.DefaultBridgingSlippageDecimal
	}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, sourceToken.ChainID, destToken.ChainID); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	tempAccount, sourceTxHash, quote, err := hinkalBridgePrivateOut(ctx, hinkal, sourceToken, destToken, amount, slippage, feeToken)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	destChainID := destToken.ChainID
	bridgedAmount, err := waitForBridgeArrival(ctx, destChainID, tempAccount.EthAddress, destToken.Erc20TokenAddress, quote, slippage)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, destChainID); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	destTxHash, pendingEnclaveUtxo, err := hinkalShieldBridgedFunds(ctx, hinkal, destToken, tempAccount, bridgedAmount, recipient)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}

	sourceChainID := sourceToken.ChainID
	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, sourceChainID)
	if err != nil {
		log.Printf("bridgePrivateToPrivate: failed to resolve ethereum address for temp-wallet nonce cleanup: %v", err)
	} else {
		hashedEthereumAddress := utils.HashEthereumAddress(ethereumAddress)
		if _, err := api.RemoveTemporaryWalletNonce(ctx, sourceChainID, hashedEthereumAddress, tempAccount.Index); err != nil {
			log.Printf("bridgePrivateToPrivate: failed to remove temp-wallet nonce on chain %d: %v", sourceChainID, err)
		}
		if _, err := api.RemoveTemporaryWalletNonce(ctx, destChainID, hashedEthereumAddress, tempAccount.Index); err != nil {
			log.Printf("bridgePrivateToPrivate: failed to remove temp-wallet nonce on chain %d: %v", destChainID, err)
		}
	}

	return bridging.PrivateBridgeResult{SourceTxHash: sourceTxHash, DestTxHash: destTxHash, PendingEnclaveUtxo: pendingEnclaveUtxo}, nil
}
