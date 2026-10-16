package transactions

import (
	"context"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func hinkalProxySend(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amounts []*big.Int,
	subAccount types.TemporarySubAccount,
	recipientAddress string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}
	ops, err := privatewallet.CreateTransferEmporiumOpsBatch(chainID, erc20Addresses, amounts, recipientAddress)
	if err != nil {
		return "", err
	}

	emporiumTokenChanges := make([]bridging.TokenChange, len(erc20Tokens))
	for index, token := range erc20Tokens {
		emporiumTokenChanges[index] = bridging.TokenChange{Token: token, Amount: new(big.Int).Neg(amounts[index])}
	}

	return actionPrivateWallet(
		ctx,
		hinkal,
		chainID,
		[]types.ERC20Token{},
		[]*big.Int{},
		[]bool{},
		ops,
		emporiumTokenChanges,
		&subAccount,
		feeToken,
		feeStructureOverride,
		"",
		action,
		nil,
	)
}
