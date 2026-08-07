package transactions

import (
	"context"
	"log"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func emitDepositAdminData(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	action types.AdminTransactionType,
	chainID int,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
) {
	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		log.Printf("emit admin data: get ethereum address error: %v", err)
		return
	}
	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}
	adminData := pretransaction.ConstructAdminData(action, chainID, erc20Addresses, amountChanges, ethereumAddress, nil)
	if adminData == nil {
		return
	}
	if err := api.EmitTxPublicData(ctx, adminData); err != nil {
		log.Printf("emit tx public data error: %v", err)
	}
}
