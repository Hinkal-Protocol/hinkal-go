package transactions

import (
	"context"
	"fmt"
	"math/big"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func getNearBridgeLegs(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	params types.NearBridgeParams,
	txCompletionTime *int,
) ([]types.NearBridgeLeg, error) {
	if len(recipientAmounts) != len(recipientAddresses) {
		return nil, errRecipientAmountLengthMismatch
	}
	if len(recipientAmounts) == 0 {
		return nil, errNearBridgeNoRecipients
	}
	if _, err := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token}); err != nil {
		return nil, err
	}

	executionTimeMs := nearBridgeExecutionTimeMs(txCompletionTime)
	deadline := nearBridgeDeadline(executionTimeMs)
	slippageBps := constants.NearBridgeSlippageBPS
	if params.SlippageBps != nil {
		slippageBps = *params.SlippageBps
	}
	refundTo, err := hinkal.GetUserKeys().GetNearIntentsAccountID()
	if err != nil {
		return nil, err
	}

	legs := make([]types.NearBridgeLeg, len(recipientAddresses))
	group, groupCtx := errgroup.WithContext(ctx)
	for i, recipientAddress := range recipientAddresses {
		i, recipientAddress := i, recipientAddress
		group.Go(func() error {
			destinationRecipient, err := toBase58IfTron(params.DestinationChainID, recipientAddress)
			if err != nil {
				return err
			}
			return getNearBridgeLeg(groupCtx, legs, i, destinationRecipient, recipientAmounts[i], types.NearIntentsQuoteRequest{
				Dry:               false,
				SwapType:          "EXACT_INPUT",
				SlippageTolerance: slippageBps,
				OriginAsset:       params.OriginAsset,
				DepositType:       "ORIGIN_CHAIN",
				DestinationAsset:  params.DestinationAsset,
				Amount:            recipientAmounts[i].String(),
				Recipient:         destinationRecipient,
				RecipientType:     "DESTINATION_CHAIN",
				RefundTo:          refundTo,
				RefundType:        "INTENTS",
				Deadline:          deadline,
			}, executionTimeMs)
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	return legs, nil
}

func getNearBridgeLeg(
	ctx context.Context,
	legs []types.NearBridgeLeg,
	index int,
	destinationRecipient string,
	amount *big.Int,
	request types.NearIntentsQuoteRequest,
	executionTimeMs int64,
) error {
	quoteResp, err := api.GetNearIntentsQuote(ctx, request)
	if err != nil {
		return err
	}
	if quoteResp.Quote.DepositAddress == "" {
		return fmt.Errorf("NEAR Intents quote returned no deposit address for recipient %s", destinationRecipient)
	}
	if err := validateNearBridgeQuoteDeadline(quoteResp.Quote, executionTimeMs); err != nil {
		return err
	}
	legs[index] = types.NearBridgeLeg{
		DestinationRecipient: destinationRecipient,
		Amount:               new(big.Int).Set(amount),
		DepositAddress:       quoteResp.Quote.DepositAddress,
		Quote:                quoteResp.Quote,
	}
	return nil
}
