package pretransaction

import (
	"context"
	"log"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func ProcessGasEstimates(
	ctx context.Context,
	chainID int,
	tokenForGasFee types.ERC20Token,
	externalActionID types.ExternalActionID,
	erc20TokenAddressLength int,
	gasAmount *int,
	calls []types.CallInfo,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (priceOfTransactionInUSD *float64, priceOfTransactionInToken *big.Int, err error) {
	apiCalls := make([]api.GasEstimateCall, len(calls))
	for i, c := range calls {
		value := "0"
		if c.Value != nil {
			value = c.Value.String()
		}
		apiCalls[i] = api.GasEstimateCall{From: c.From, To: c.To, Calldata: c.Calldata, Value: value}
	}

	resp, gasErr := api.GetGasEstimates(ctx, chainID, tokenForGasFee.Erc20TokenAddress, string(externalActionID), erc20TokenAddressLength, apiCalls, gasAmount, solanaTransactionParams)
	if gasErr != nil {
		log.Printf("processGasEstimates error: %v", gasErr)
		return nil, nil, nil
	}

	if resp.PriceOfTransactionInToken != nil && *resp.PriceOfTransactionInToken != 0 {
		weiInput := utils.GetValueFirstNDigit(*resp.PriceOfTransactionInToken, tokenForGasFee.Decimals)
		priceOfTransactionInToken, err = web3.GetAmountInWei(tokenForGasFee, weiInput)
		if err != nil {
			return nil, nil, err
		}
	}

	return resp.PriceOfTransactionInUSD, priceOfTransactionInToken, nil
}
