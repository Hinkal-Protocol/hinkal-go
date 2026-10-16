package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errNearBridgeNoRecipients = errors.New("no recipients to bridge")

func nearBridgeExecutionTimeMs(txCompletionTime *int) int64 {
	if txCompletionTime == nil {
		return time.Now().UnixMilli()
	}
	return int64(*txCompletionTime) * 1000
}

func nearBridgeDeadline(executionTimeMs int64) string {
	deadlineMs := executionTimeMs + constants.NearBridgeQuoteDeadlineBufferMS
	return time.UnixMilli(deadlineMs).UTC().Format("2006-01-02T15:04:05.000Z")
}

func validateNearBridgeQuoteDeadline(quote types.NearIntentsQuote, executionTimeMs int64) error {
	if quote.Deadline == "" {
		return nil
	}
	deadline, err := time.Parse(time.RFC3339Nano, quote.Deadline)
	if err != nil {
		return fmt.Errorf("transactions: invalid NEAR Intents quote deadline %q: %w", quote.Deadline, err)
	}
	if deadline.UnixMilli() < executionTimeMs {
		return errors.New("bridge quote expires before the scheduled execution time. Choose a sooner time.")
	}
	return nil
}

func buildNearBridgeFeeStructure(
	ctx context.Context,
	chainID int,
	token types.ERC20Token,
	recipient string,
) (types.FeeStructure, error) {
	return pretransaction.GetFeeStructure(
		ctx,
		chainID,
		token.Erc20TokenAddress,
		[]string{token.Erc20TokenAddress},
		types.ExternalActionTransact,
		[]types.CallInfo{},
		constants.PaySendVariableRate(),
		&api.SolanaGasEstimateParams{
			MintTo:         token.Erc20TokenAddress,
			Recipient:      recipient,
			NullifierCount: 1,
		},
	)
}

// hinkalNearDepositAndBridgeLegs mirrors hinkalNearDepositAndBridgeLegs (hinkalNearDepositAndBridge.ts).
func hinkalNearDepositAndBridgeLegs(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	legs []types.NearBridgeLeg,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (types.DepositAndSendExtendedResult, error) {
	if len(legs) == 0 {
		return types.DepositAndSendExtendedResult{}, errNearBridgeNoRecipients
	}

	erc20Tokens := []types.ERC20Token{token}
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	executionTimeMs := nearBridgeExecutionTimeMs(txCompletionTime)
	for _, leg := range legs {
		if err := validateNearBridgeQuoteDeadline(leg.Quote, executionTimeMs); err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
	}

	feeStructure := feeStructureOverride
	if feeStructure == nil {
		computedFeeStructure, err := buildNearBridgeFeeStructure(ctx, chainID, token, legs[0].DepositAddress)
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
		feeStructure = &computedFeeStructure
	}

	amounts := make([]*big.Int, len(legs))
	depositAddresses := make([]string, len(legs))
	for i, leg := range legs {
		amounts[i] = new(big.Int).Set(leg.Amount)
		depositAddresses[i] = leg.DepositAddress
	}

	if constants.IsSolanaLike(chainID) {
		return HinkalSolanaDepositAndWithdraw(
			ctx,
			hinkal,
			erc20Tokens,
			amounts,
			depositAddresses,
			txCompletionTime,
			feeStructure,
			action,
		)
	}
	return HinkalDepositAndWithdraw(
		ctx,
		hinkal,
		erc20Tokens,
		amounts,
		depositAddresses,
		txCompletionTime,
		feeStructure,
		true,
		action,
	)
}

func HinkalNearDepositAndBridge(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	params types.NearBridgeParams,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	action types.AdminTransactionType,
) (types.NearBridgeResult, error) {
	if len(recipientAmounts) != len(recipientAddresses) {
		return types.NearBridgeResult{}, errRecipientAmountLengthMismatch
	}
	if len(recipientAmounts) == 0 {
		return types.NearBridgeResult{}, errNearBridgeNoRecipients
	}
	if err := validateDepositAndWithdrawArgs(erc20Tokens, recipientAmounts, recipientAddresses); err != nil {
		return types.NearBridgeResult{}, err
	}
	if _, err := pretransaction.ValidateAndGetChainID(erc20Tokens); err != nil {
		return types.NearBridgeResult{}, err
	}

	legs, err := getNearBridgeLegs(ctx, hinkal, erc20Tokens[0], recipientAmounts, recipientAddresses, params, txCompletionTime)
	if err != nil {
		return types.NearBridgeResult{}, err
	}

	result, err := hinkalNearDepositAndBridgeLegs(ctx, hinkal, erc20Tokens[0], legs, txCompletionTime, feeStructureOverride, action)
	if err != nil {
		return types.NearBridgeResult{}, err
	}

	return types.NearBridgeResult{
		DepositTxHash: result.DepositTxHash,
		ScheduleID:    result.ScheduleID,
		Legs:          legs,
	}, nil
}
