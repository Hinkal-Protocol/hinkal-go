package transactions

import (
	"context"
	"errors"
	"math"
	"math/big"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetDepositAndBridgeQuote(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken types.ERC20Token,
	destinationToken types.ERC20Token,
	sendAmounts []*big.Int,
	recipientAddresses []string,
	slippagePercent *float64, // If this is undefined, getNearBridgeLegs and getLifiBridgeRecipients will choose defualt value themself.
	txCompletionTime *int,
) (types.DepositAndBridgeQuote, error) {
	route := types.DepositAndBridgeQuote{
		SourceChainID:           sourceToken.ChainID,
		SourceTokenAddress:      sourceToken.Erc20TokenAddress,
		DestinationChainID:      destinationToken.ChainID,
		DestinationTokenAddress: destinationToken.Erc20TokenAddress,
	}

	if constants.UsesNearIntentsBridge(sourceToken.ChainID) {
		var originAsset, destinationAsset string
		group, groupCtx := errgroup.WithContext(ctx)
		group.Go(func() (err error) {
			originAsset, err = integrations.ResolveNearAssetID(groupCtx, sourceToken.ChainID, sourceToken.Erc20TokenAddress)
			return err
		})
		group.Go(func() (err error) {
			destinationAsset, err = integrations.ResolveNearAssetID(groupCtx, destinationToken.ChainID, destinationToken.Erc20TokenAddress)
			return err
		})
		if err := group.Wait(); err != nil {
			return types.DepositAndBridgeQuote{}, err
		}

		var slippageBps *int
		if slippagePercent != nil {
			bps := int(math.Max(1, math.Round(*slippagePercent*100)))
			slippageBps = &bps
		}
		nearLegs, err := getNearBridgeLegs(
			ctx,
			hinkal,
			sourceToken,
			sendAmounts,
			recipientAddresses,
			types.NearBridgeParams{
				OriginAsset:        originAsset,
				DestinationAsset:   destinationAsset,
				DestinationChainID: destinationToken.ChainID,
				SlippageBps:        slippageBps,
			},
			txCompletionTime,
		)
		if err != nil {
			return types.DepositAndBridgeQuote{}, err
		}

		route.ExpectedAmounts = make([]*big.Int, len(nearLegs))
		for i, leg := range nearLegs {
			expectedAmount, err := utils.ParseBigInt(leg.Quote.AmountOut)
			if err != nil {
				return types.DepositAndBridgeQuote{}, err
			}
			route.ExpectedAmounts[i] = expectedAmount
		}
		route.NearLegs = nearLegs
		return route, nil
	}

	if _, err := constants.LifiRouterAddress(sourceToken.ChainID); err != nil {
		return types.DepositAndBridgeQuote{}, errors.New("LIFI router not configured")
	}

	lifiRecipients, err := getLifiBridgeRecipients(
		ctx,
		hinkal,
		sourceToken,
		destinationToken,
		sendAmounts,
		recipientAddresses,
		slippagePercent,
	)
	if err != nil {
		return types.DepositAndBridgeQuote{}, err
	}

	route.ExpectedAmounts = make([]*big.Int, len(lifiRecipients))
	for i, recipient := range lifiRecipients {
		route.ExpectedAmounts[i] = recipient.Quote.ExpectedAmount
	}
	route.LifiRecipients = lifiRecipients
	return route, nil
}
