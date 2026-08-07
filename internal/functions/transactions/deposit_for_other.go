package transactions

import (
	"context"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func HinkalDepositForOther(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	recipientInfo string,
	preEstimateGas bool,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		return types.TransactionRequest{}, "", errDepositNotImplemented
	}
	if !pretransaction.IsValidPrivateAddress(recipientInfo) {
		return types.TransactionRequest{}, "", errorhandling.ErrRecipientFormatIncorrect
	}

	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}

	if returnTxData && constants.IsTronLike(chainID) {
		return types.TransactionRequest{}, "", errTronReturnTxDataNotImplemented
	}

	ethereumAddress, err := depositExternalAddress(ctx, hinkal, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	zeroAmountChanges := make([]*big.Int, len(amountChanges))
	for i := range zeroAmountChanges {
		zeroAmountChanges[i] = big.NewInt(0)
	}

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:          chainID,
		Erc20Addresses:   erc20Addresses,
		AmountChanges:    zeroAmountChanges,
		ExternalActionID: types.ExternalActionZero,
		ExternalAddress:  ethereumAddress,

		RecipientAddress: recipientInfo,
		RecipientAmounts: amountChanges,
		ForceEmptyUtxos:  true,
		Submit: NewSelfSubmit(SelfSubmit{
			Erc20Tokens:     erc20Tokens,
			ApprovalAmounts: amountChanges,
			PreEstimateGas:  preEstimateGas,
			ReturnTxData:    returnTxData,
		}),
	})
	if err != nil {
		return result.TxRequest, result.TxHash, err
	}
	emitDepositAdminData(ctx, hinkal, types.AdminShield, chainID, erc20Tokens, amountChanges)
	return result.TxRequest, result.TxHash, nil
}
