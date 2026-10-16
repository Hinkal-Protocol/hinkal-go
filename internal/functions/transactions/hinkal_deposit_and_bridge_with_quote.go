package transactions

import (
	"context"
	"errors"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func HinkalDepositAndBridgeWithQuote(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	quote types.DepositAndBridgeQuote,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (types.DepositAndSendExtendedResult, error) {
	if quote.SourceChainID != token.ChainID || !strings.EqualFold(quote.SourceTokenAddress, token.Erc20TokenAddress) {
		return types.DepositAndSendExtendedResult{}, errors.New("Quote was issued for a different chain or token")
	}

	lifiRecipients, nearLegs := quote.LifiRecipients, quote.NearLegs
	if len(lifiRecipients) > 0 && len(nearLegs) > 0 {
		return types.DepositAndSendExtendedResult{}, errors.New("Quote must contain either lifiRecipients or nearLegs, not both")
	}

	if len(lifiRecipients) > 0 {
		return HinkalDepositAndBridge(
			ctx,
			hinkal,
			[]types.ERC20Token{token},
			lifiRecipients,
			txCompletionTime,
			feeStructureOverride,
			true,
			action,
		)
	}

	return hinkalNearDepositAndBridgeLegs(
		ctx,
		hinkal,
		token,
		nearLegs,
		txCompletionTime,
		feeStructureOverride,
		action,
	)
}
