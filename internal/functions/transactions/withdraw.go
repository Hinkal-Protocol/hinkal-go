package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errWithdrawNoToken = errors.New("transactions: withdraw action: no token found")

func copyBigInts(values []*big.Int) []*big.Int {
	out := make([]*big.Int, len(values))
	for i, value := range values {
		out[i] = new(big.Int).Set(value)
	}
	return out
}

func tokenAddresses(tokens []types.ERC20Token) []string {
	addresses := make([]string, len(tokens))
	for i, token := range tokens {
		addresses[i] = token.Erc20TokenAddress
	}
	return addresses
}

func resolveWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	erc20Addresses []string,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return normalizeFeeStructure(*feeStructureOverride), nil
	}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, feeToken, erc20Addresses, types.ExternalActionTransact, nil, nil, solanaTransactionParams)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return normalizeFeeStructure(feeStructure), nil
}

func relayerAddress(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int) (string, error) {
	relay, err := hinkal.GetRandomRelay(ctx, chainID, true)
	if err != nil {
		return "", err
	}
	if relay == "" {
		return "", errorhandling.ErrRelayerNotAvailable
	}
	return relay, nil
}

func HinkalWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	recipientAddress string,
	isRelayerOff bool,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (types.TransactionRequest, string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if len(erc20Tokens) != len(amountChangesBase) {
		return types.TransactionRequest{}, "", errTokenAmountLengthMismatch
	}
	if len(erc20Tokens) == 0 {
		return types.TransactionRequest{}, "", errWithdrawNoToken
	}

	amountChanges := pretransaction.ModifyVolatileTokenAmountChanges(ctx, chainID, erc20Tokens, copyBigInts(amountChangesBase), "")
	erc20Addresses := tokenAddresses(erc20Tokens)
	token := erc20Tokens[0]

	feeStructure := types.ZeroFeeStructure()
	if !isRelayerOff {
		feeStructure, err = resolveWithdrawFeeStructure(ctx, chainID, feeToken, erc20Addresses, feeStructureOverride, nil)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		if err := pretransaction.MergeWithFeeStructure(ctx, chainID, &erc20Addresses, &amountChanges, feeStructure); err != nil {
			return types.TransactionRequest{}, "", err
		}
	}

	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	originalSender := ""
	if isRelayerOff {
		originalSender = ethereumAddress
		if constants.IsTronLike(chainID) {
			originalSender, err = utils.AddressToHexFormat(ethereumAddress)
			if err != nil {
				return types.TransactionRequest{}, "", err
			}
		}
	}

	relay := constants.ZeroAddress
	if !isRelayerOff {
		relay, err = relayerAddress(ctx, hinkal, chainID)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
	}

	externalAddress, err := utils.AddressToHexFormat(recipientAddress)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	submit := NewRelayerSubmit(RelayerSubmit{
		AdminData: pretransaction.ConstructAdminData(types.AdminUnshield, chainID, erc20Addresses, amountChanges, ethereumAddress, nil),
	})
	if isRelayerOff {
		submit = NewSelfSubmit(SelfSubmit{
			Erc20Tokens:     []types.ERC20Token{token},
			ApprovalAmounts: []*big.Int{amountChanges[0]},
			PreEstimateGas:  true,
		})
	}

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:          chainID,
		Erc20Addresses:   erc20Addresses,
		AmountChanges:    amountChanges,
		ExternalActionID: types.ExternalActionZero,
		ExternalAddress:  externalAddress,
		FeeStructure:     &feeStructure,
		Relay:            relay,
		OriginalSender:   originalSender,
		Submit:           submit,
	})
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return result.TxRequest, result.TxHash, nil
}
