package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
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
	nullifierCount := 1
	if strings.EqualFold(token.Erc20TokenAddress, constants.SolanaNativeAddress) {
		nullifierCount = 0
	}
	return pretransaction.GetFeeStructure(
		ctx,
		chainID,
		token.Erc20TokenAddress,
		[]string{token.Erc20TokenAddress},
		types.ExternalActionTransact,
		[]types.CallInfo{},
		constants.HinkalPrivateSendVariableRate(),
		&api.SolanaGasEstimateParams{
			MintTo:         token.Erc20TokenAddress,
			Recipient:      recipient,
			NullifierCount: nullifierCount,
		},
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
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.NearBridgeResult{}, err
	}

	executionTimeMs := nearBridgeExecutionTimeMs(txCompletionTime)
	deadline := nearBridgeDeadline(executionTimeMs)
	slippageBps := constants.NearBridgeSlippageBPS
	if params.SlippageBps != nil {
		slippageBps = *params.SlippageBps
	}
	refundTo, err := hinkal.GetUserKeys().GetNearIntentsAccountID()
	if err != nil {
		return types.NearBridgeResult{}, err
	}

	legs := make([]types.NearBridgeLeg, len(recipientAddresses))
	for i, destinationRecipient := range recipientAddresses {
		amount := recipientAmounts[i]
		quoteResp, err := api.GetNearIntentsQuote(ctx, types.NearIntentsQuoteRequest{
			Dry:               false,
			SwapType:          "EXACT_INPUT",
			SlippageTolerance: slippageBps,
			OriginAsset:       params.OriginAsset,
			DepositType:       "ORIGIN_CHAIN",
			DestinationAsset:  params.DestinationAsset,
			Amount:            amount.String(),
			Recipient:         destinationRecipient,
			RecipientType:     "DESTINATION_CHAIN",
			RefundTo:          refundTo,
			RefundType:        "INTENTS",
			Deadline:          deadline,
		})
		if err != nil {
			return types.NearBridgeResult{}, err
		}
		if quoteResp.Quote.DepositAddress == "" {
			return types.NearBridgeResult{}, fmt.Errorf("NEAR Intents quote returned no deposit address for recipient %s", destinationRecipient)
		}
		if err := validateNearBridgeQuoteDeadline(quoteResp.Quote, executionTimeMs); err != nil {
			return types.NearBridgeResult{}, err
		}
		legs[i] = types.NearBridgeLeg{
			DestinationRecipient: destinationRecipient,
			Amount:               new(big.Int).Set(amount),
			DepositAddress:       quoteResp.Quote.DepositAddress,
			Quote:                quoteResp.Quote,
		}
	}

	feeStructure := feeStructureOverride
	if feeStructure == nil {
		computedFeeStructure, err := buildNearBridgeFeeStructure(ctx, chainID, erc20Tokens[0], legs[0].DepositAddress)
		if err != nil {
			return types.NearBridgeResult{}, err
		}
		feeStructure = &computedFeeStructure
	}

	amounts := make([]*big.Int, len(legs))
	depositAddresses := make([]string, len(legs))
	for i, leg := range legs {
		amounts[i] = new(big.Int).Set(leg.Amount)
		depositAddresses[i] = leg.DepositAddress
	}

	var result types.DepositAndSendExtendedResult
	if constants.IsSolanaLike(chainID) {
		result, err = HinkalSolanaDepositAndWithdraw(
			ctx,
			hinkal,
			erc20Tokens,
			amounts,
			depositAddresses,
			txCompletionTime,
			feeStructure,
		)
	} else {
		result, err = HinkalDepositAndWithdraw(
			ctx,
			hinkal,
			erc20Tokens,
			amounts,
			depositAddresses,
			txCompletionTime,
			feeStructure,
			true,
		)
	}
	if err != nil {
		return types.NearBridgeResult{}, err
	}

	return types.NearBridgeResult{
		DepositTxHash: result.DepositTxHash,
		Legs:          legs,
	}, nil
}
