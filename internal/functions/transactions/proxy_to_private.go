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

func hinkalProxyToPrivate(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	destToken types.ERC20Token,
	amount *big.Int,
	subAccount types.TemporarySubAccount,
	recipientInfo string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (string, error) {
	transferOps, err := privatewallet.CreateTransferEmporiumOpsBatch(chainID, []string{destToken.Erc20TokenAddress}, []*big.Int{amount}, "")
	if err != nil {
		return "", err
	}

	feeStructure := feeStructureOverride
	if feeStructure == nil {
		calls := make([]types.CallInfo, len(transferOps))
		for i, op := range transferOps {
			call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, subAccount.EthAddress, chainID)
			if err != nil {
				return "", err
			}
			calls[i] = call
		}
		resolved, err := pretransaction.GetFeeStructure(ctx, chainID, feeToken, []string{destToken.Erc20TokenAddress}, types.ExternalActionEmporium, calls, nil, nil)
		if err != nil {
			return "", err
		}
		feeStructure = &resolved
	}

	emporiumTokenChanges := []bridging.TokenChange{{Token: destToken, Amount: new(big.Int).Neg(amount)}}
	privateRecipientInfo := &bridging.PrivateRecipientInfo{RecipientInfo: recipientInfo, Amount: amount, Token: destToken}

	return actionPrivateWallet(
		ctx,
		hinkal,
		chainID,
		[]types.ERC20Token{destToken},
		[]*big.Int{big.NewInt(0)},
		[]bool{false},
		transferOps,
		emporiumTokenChanges,
		&subAccount,
		feeToken,
		feeStructure,
		"",
		action,
		privateRecipientInfo,
	)
}
