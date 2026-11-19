package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type OKXQuote struct {
	Amount            string `json:"amount"`
	ChainIndex        string `json:"chainIndex"`
	FromTokenAddress  string `json:"fromTokenAddress"`
	ToTokenAddress    string `json:"toTokenAddress"`
	UserWalletAddress string `json:"userWalletAddress"`
	SlippagePercent   string `json:"slippagePercent"`
	DirectRoute       bool   `json:"directRoute"`
}

type OKXSwapResponseRouterResult struct {
	FromTokenAmount string `json:"fromTokenAmount"`
	ToTokenAmount   string `json:"toTokenAmount"`
}

type OKXSwapResponseInstruction struct {
	Data      string       `json:"data"`
	Accounts  []OKXAccount `json:"accounts"`
	ProgramID string       `json:"programId"`
}

type OKXSwapResponseData struct {
	AddressLookupTableAccount []string                     `json:"addressLookupTableAccount"`
	InstructionLists          []OKXSwapResponseInstruction `json:"instructionLists"`
	RouterResult              OKXSwapResponseRouterResult  `json:"routerResult"`
}

type OKXSwapResponse struct {
	Code string              `json:"code"`
	Data OKXSwapResponseData `json:"data"`
	Msg  string              `json:"msg"`
}

type okxSwapRequest struct {
	Quote OKXQuote `json:"quote"`
}

type callOkxAPIResponse struct {
	OkxResponse OKXSwapResponse `json:"okxResponse"`
	Status      string          `json:"status"`
	Message     string          `json:"message"`
}

type SolanaSwapDataRequest struct {
	ChainID          int    `json:"chainId"`
	FromTokenAddress string `json:"fromTokenAddress"`
	ToTokenAddress   string `json:"toTokenAddress"`
	Amount           string `json:"amount"`
	SlippagePercent  string `json:"slippagePercent"`
}

type SolanaSwapDataResponse struct {
	Status             string          `json:"status"`
	OkxResponse        OKXSwapResponse `json:"okxResponse"`
	SwapperAccountSalt string          `json:"swapperAccountSalt"`
	RouteTag           string          `json:"routeTag"`
	Error              string          `json:"error"`
}

// CallSolanaSwapData gets the OKX route for a Hinkal swap from henclave, which only relays
// swaps whose route it fetched itself.
func CallSolanaSwapData(ctx context.Context, req SolanaSwapDataRequest) (SolanaSwapDataResponse, error) {
	var resp SolanaSwapDataResponse
	url := constants.GetEnclaveURL() + constants.RelayerConfig.SolanaSwapData
	if err := Post(ctx, url, req, &resp); err != nil {
		return SolanaSwapDataResponse{}, err
	}
	return resp, nil
}

func CallOkxAPI(ctx context.Context, quote OKXQuote) (OKXSwapResponse, string, error) {
	var resp callOkxAPIResponse
	url := constants.GetServerURL() + constants.ServerConfig.CallOkxAPI
	if err := Post(ctx, url, okxSwapRequest{Quote: quote}, &resp); err != nil {
		return OKXSwapResponse{}, "", err
	}
	return resp.OkxResponse, resp.Status, nil
}
