package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errClaimNoToken                = errors.New("transactions: claimUtxo action: no token found")
	errClaimTooManyTokens          = errors.New("transactions: claimUtxo supports one token")
	errClaimUtxoMissingKey         = errors.New("claimable UTXO nullifyingKey is missing")
	errClaimUtxoKeyMismatch        = errors.New("claimable UTXO key mismatch")
	errClaimUtxoTokenMismatch      = errors.New("off-chain UTXO token mismatch")
	errClaimFeeTokenMismatch       = errors.New("claim fee token mismatch: fee must be paid from claimed UTXO token")
	errClaimNewStyleNeedsSignature = errors.New("claimable new-style UTXO requires claimableSignature")
)

func claimUtxoUserKeys(source *utxo.Utxo, claimableSignature string) (*cryptokeys.UserKeys, string, error) {
	if claimableSignature != "" {
		keys := cryptokeys.NewUserKeys(claimableSignature)
		resolved, err := keys.GetShieldedPrivateKey()
		return keys, resolved, err
	}
	if source.NullifyingKey == "" {
		return nil, "", errClaimUtxoMissingKey
	}
	return nil, "", errClaimNewStyleNeedsSignature
}

func claimFeeParts(sourceAmount *big.Int, feeStructure types.FeeStructure) (types.FeeStructure, *big.Int, error) {
	flatFee := feeStructure.FlatFee
	if flatFee == nil {
		flatFee = big.NewInt(0)
	}
	variableRate := feeStructure.VariableRate
	if variableRate == nil || variableRate.Sign() <= 0 {
		variableRate = constants.HinkalPrivateSendVariableRate()
	}
	if sourceAmount.Cmp(flatFee) <= 0 {
		return types.FeeStructure{}, nil, errors.New(errorhandling.ErrCodeInsufficientFundsToTransact)
	}

	transferableAmount := new(big.Int).Sub(sourceAmount, flatFee)
	variableFee := new(big.Int).Div(new(big.Int).Mul(transferableAmount, variableRate), big.NewInt(10000))
	recipientAmount := new(big.Int).Sub(transferableAmount, variableFee)
	if recipientAmount.Sign() <= 0 {
		return types.FeeStructure{}, nil, errorhandling.ErrRecipientAmountInvalid
	}

	return types.FeeStructure{
		FeeToken:     feeStructure.FeeToken,
		FlatFee:      new(big.Int).Add(flatFee, variableFee),
		VariableRate: big.NewInt(0),
	}, recipientAmount, nil
}

func claimPaddingUtxo(source *utxo.Utxo, keys *cryptokeys.UserKeys, resolvedNullifyingKey string) (*utxo.Utxo, error) {
	params := types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: source.Erc20TokenAddress,
		MintAddress:       source.MintAddress,
		NullifyingKey:     resolvedNullifyingKey,
	}
	spendingKeyPair, err := keys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	params.SpendingPublicKey = []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	return utxo.NewUtxo(params)
}

func HinkalClaimUtxo(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	claimableUtxo *utxo.Utxo,
	feeStructureOverride *types.FeeStructure,
	claimableSignature string,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if len(erc20Tokens) == 0 {
		return "", errClaimNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errClaimTooManyTokens
	}
	if claimableUtxo == nil || claimableUtxo.Amount == nil {
		return "", errClaimUtxoMissingKey
	}

	tokenAddress := erc20Tokens[0].Erc20TokenAddress
	if !strings.EqualFold(tokenAddress, claimableUtxo.Erc20TokenAddress) {
		return "", errClaimUtxoTokenMismatch
	}

	utxoSpecificUserKeys, resolvedNullifyingKey, err := claimUtxoUserKeys(claimableUtxo, claimableSignature)
	if err != nil {
		return "", err
	}
	if claimableUtxo.NullifyingKey != "" {
		storedKey, err := utils.ParseBigInt(claimableUtxo.NullifyingKey)
		if err != nil {
			return "", err
		}
		resolvedKey, err := utils.ParseBigInt(resolvedNullifyingKey)
		if err != nil {
			return "", err
		}
		if storedKey.Cmp(resolvedKey) != 0 {
			return "", errClaimUtxoKeyMismatch
		}
	}

	sourceUtxo, err := utxo.CreateFrom(claimableUtxo, types.UtxoParams{NullifyingKey: resolvedNullifyingKey})
	if err != nil {
		return "", err
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, tokenAddress, []string{tokenAddress}, types.ExternalActionTransact, nil, nil, nil)
		if err != nil {
			return "", err
		}
	}
	feeStructure = normalizeFeeStructure(feeStructure)
	if !strings.EqualFold(feeStructure.FeeToken, tokenAddress) {
		return "", errClaimFeeTokenMismatch
	}

	claimFeeStructure, recipientAmount, err := claimFeeParts(sourceUtxo.Amount, feeStructure)
	if err != nil {
		return "", err
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}

	recipientInfo, err := hinkal.GetRecipientInfo()
	if err != nil {
		return "", err
	}
	amountChange := new(big.Int).Neg(sourceUtxo.Amount)

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:          chainID,
		Erc20Addresses:   []string{tokenAddress},
		AmountChanges:    []*big.Int{amountChange},
		ExternalActionID: types.ExternalActionZero,
		ExternalAddress:  relay,
		FeeStructure:     &claimFeeStructure,
		Relay:            relay,
		RecipientAddress: recipientInfo,
		RecipientAmounts: []*big.Int{recipientAmount},
		InputUtxos:       [][]*utxo.Utxo{{sourceUtxo}},
		UserKeys:         utxoSpecificUserKeys,
		Submit:           NewRelayerSubmit(RelayerSubmit{}),
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
