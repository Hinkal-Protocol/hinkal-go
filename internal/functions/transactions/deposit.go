package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errDepositNotImplemented          = errors.New("transactions: deposit is not implemented for Solana chains")
	errTronReturnTxDataNotImplemented = errors.New("transactions: Tron returnTxData is not implemented")
)

func depositExternalAddress(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int) (string, error) {
	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return "", err
	}
	return utils.AddressToHexFormat(ethereumAddress)
}

func HinkalDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
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

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:          chainID,
		Erc20Addresses:   erc20Addresses,
		AmountChanges:    amountChanges,
		ExternalActionID: types.ExternalActionZero,
		ExternalAddress:  ethereumAddress,

		Submit: NewSelfSubmit(SelfSubmit{
			Erc20Tokens:     erc20Tokens,
			ApprovalAmounts: amountChanges,
			PreEstimateGas:  preEstimateGas,
			ReturnTxData:    returnTxData,
		}),
	})
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return result.TxRequest, result.TxHash, nil
}
