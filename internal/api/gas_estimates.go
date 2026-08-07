package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type GasEstimateCall struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Calldata string `json:"calldata"`
	Value    string `json:"value"`
}

type GetGasEstimatesResponse struct {
	Status                    string   `json:"status"`
	PriceOfTransactionInUSD   *float64 `json:"priceOfTransactionInUSD"`
	PriceOfTransactionInToken *float64 `json:"priceOfTransactionInToken"`
}

func GetGasEstimates(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	externalActionID string,
	erc20TokenAddressLength int,
	calls []GasEstimateCall,
	gasAmount *int,
	solanaTransactionParams *SolanaGasEstimateParams,
) (*GetGasEstimatesResponse, error) {
	url := constants.GetRelayerURL() + constants.RelayerConfig.GetGasEstimate(tokenAddress, externalActionID, erc20TokenAddressLength, gasAmount)
	body := map[string]any{
		"calls":                   calls,
		"solanaTransactionParams": solanaTransactionParams,
		"chainId":                 chainID,
	}
	var resp GetGasEstimatesResponse
	if err := Post(ctx, url, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
