package transactions

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func getNearIntentsRefundBalances(ctx context.Context, hinkal ihinkal.HinkalInternal) []types.NearIntentsRefundBalance {
	balances, err := fetchNearIntentsRefundBalances(ctx, hinkal.GetUserKeys())
	if err != nil {
		log.Printf("Error loading NEAR Intents refund balances: %v", err)
		return []types.NearIntentsRefundBalance{}
	}

	priced := make([]types.NearIntentsRefundBalance, len(balances))
	var wg sync.WaitGroup
	for index, refund := range balances {
		index, refund := index, refund
		wg.Add(1)
		go func() {
			defer wg.Done()
			prices, err := api.GetTokenPrices(ctx, refund.Token.ChainID, []string{refund.Token.Erc20TokenAddress})
			if err != nil {
				log.Printf("Error pricing NEAR Intents refund balance: %v", err)
				priced[index] = refund
				return
			}
			price := 0.0
			if len(prices.Prices) > 0 {
				price = prices.Prices[0]
			}
			balanceInToken, _ := strconv.ParseFloat(web3.GetAmountInToken(refund.Token, refund.Balance), 64)
			refund.UsdValue = balanceInToken * price
			refund.IsBalanceLoading = false
			refund.IsUsdValueLoading = false
			priced[index] = refund
		}()
	}
	wg.Wait()
	return priced
}

func startNearIntentsRefundBalanceClaim(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	balance types.NearIntentsRefundBalance,
	recipientAddressOverride string,
) (func(context.Context) error, error) {
	destinationAddress := strings.TrimSpace(recipientAddressOverride)
	if destinationAddress == "" {
		if constants.IsSolanaLike(balance.Token.ChainID) {
			solanaPublicKey, err := hinkal.GetSolanaPublicKey(ctx)
			if err != nil {
				return nil, err
			}
			destinationAddress = solanaPublicKey.String()
		} else {
			ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
			if err != nil {
				return nil, err
			}
			destinationAddress = ethereumAddress
		}
	}

	return startNearIntentsRefundClaim(ctx, claimNearIntentsRefundParams{
		UserKeys:           hinkal.GetUserKeys(),
		AssetID:            balance.AssetID,
		Amount:             balance.Balance,
		DestinationAddress: destinationAddress,
		IsLegacy:           balance.IsLegacy,
	})
}
