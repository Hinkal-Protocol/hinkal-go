package web3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errOKXSolanaOnly = errors.New("OKX is only supported on Solana")
	errOKXFetch      = errors.New("OKX API Fetch Error")
)

type OKXPrice struct {
	OutSwapAmount *big.Int
	OKXData       string
}

func GetOKXPrice(
	ctx context.Context,
	chainID int,
	inSwapToken, outSwapToken types.ERC20Token,
	inSwapAmount string,
	slippagePercentage float64,
) (OKXPrice, error) {
	if !constants.IsSolanaLike(chainID) {
		return OKXPrice{}, errOKXSolanaOnly
	}

	inSwapAmountWei, err := GetAmountInWei(inSwapToken, inSwapAmount)
	if err != nil {
		return OKXPrice{}, err
	}

	// The relayer only relays Hinkal swaps whose route it fetched itself, so the route comes from it.
	route, err := api.CallSolanaSwapData(ctx, api.SolanaSwapDataRequest{
		ChainID:          chainID,
		FromTokenAddress: inSwapToken.Erc20TokenAddress,
		ToTokenAddress:   outSwapToken.Erc20TokenAddress,
		Amount:           inSwapAmountWei.String(),
		SlippagePercent:  fmt.Sprintf("%v", slippagePercentage),
	})
	if err != nil {
		return OKXPrice{}, err
	}
	if route.Status != "success" {
		if route.Error != "" {
			return OKXPrice{}, errors.New(route.Error)
		}
		return OKXPrice{}, errOKXFetch
	}
	okxResponse := route.OkxResponse
	if okxResponse.Code != "0" {
		return OKXPrice{}, fmt.Errorf("OKX API error: %s", okxResponse.Msg)
	}

	outSwapAmount, ok := new(big.Int).SetString(okxResponse.Data.RouterResult.ToTokenAmount, 10)
	if !ok {
		return OKXPrice{}, errOKXFetch
	}

	okxData, err := json.Marshal(struct {
		api.OKXSwapResponse
		SwapperAccountSalt string `json:"swapperAccountSalt"`
		RouteTag           string `json:"routeTag"`
	}{OKXSwapResponse: okxResponse, SwapperAccountSalt: route.SwapperAccountSalt, RouteTag: route.RouteTag})
	if err != nil {
		return OKXPrice{}, err
	}

	return OKXPrice{OutSwapAmount: outSwapAmount, OKXData: string(okxData)}, nil
}
