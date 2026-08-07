package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type getOdosPriceForTokenResponse struct {
	Price float64 `json:"price"`
}

func GetOdosPriceForToken(ctx context.Context, chainID int, tokenAddress string) (float64, error) {
	var resp getOdosPriceForTokenResponse
	url := constants.GetServerURL() + constants.ServerConfig.GetOdosPriceForToken(chainID, tokenAddress)
	if err := Get(ctx, url, &resp); err != nil {
		return 0, err
	}
	return resp.Price, nil
}
