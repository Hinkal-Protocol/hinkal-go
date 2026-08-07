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

func CallOkxAPI(ctx context.Context, quote OKXQuote) (OKXSwapResponse, string, error) {
	var resp callOkxAPIResponse
	url := constants.GetServerURL() + constants.ServerConfig.CallOkxAPI
	if err := Post(ctx, url, okxSwapRequest{Quote: quote}, &resp); err != nil {
		return OKXSwapResponse{}, "", err
	}
	return resp.OkxResponse, resp.Status, nil
}
