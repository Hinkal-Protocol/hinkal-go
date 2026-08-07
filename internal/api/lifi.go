package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type StringOrNumber string

func (s *StringOrNumber) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(raw, []byte("null")) {
		*s = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		*s = StringOrNumber(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		*s = StringOrNumber(num.String())
		return nil
	}
	return fmt.Errorf("api: expected string or number, got %s", raw)
}

func (s StringOrNumber) String() string {
	return string(s)
}

type LifiRequestData struct {
	FromChain   int     `json:"fromChain"`
	ToChain     int     `json:"toChain"`
	FromToken   string  `json:"fromToken"`
	ToToken     string  `json:"toToken"`
	FromAddress string  `json:"fromAddress"`
	ToAddress   string  `json:"toAddress,omitempty"`
	FromAmount  string  `json:"fromAmount"`
	Order       string  `json:"order"`
	Slippage    float64 `json:"slippage"`
}

type LifiBridgeResponse struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Tool               string `json:"tool"`
	TransactionRequest struct {
		From    string         `json:"from"`
		To      string         `json:"to"`
		ChainID StringOrNumber `json:"chainId"`
		Value   StringOrNumber `json:"value"`
		Data    string         `json:"data"`
	} `json:"transactionRequest"`
	Estimate struct {
		Tool        string         `json:"tool"`
		FromAmount  StringOrNumber `json:"fromAmount"`
		ToAmount    StringOrNumber `json:"toAmount"`
		ToAmountMin StringOrNumber `json:"toAmountMin"`
	} `json:"estimate"`
}

type callLifiAPIResponse struct {
	Status       string             `json:"status"`
	LifiResponse LifiBridgeResponse `json:"lifiResponse"`
}

func CallLifiAPI(ctx context.Context, request LifiRequestData) (LifiBridgeResponse, string, error) {
	var resp callLifiAPIResponse
	url := constants.GetServerURL() + constants.ServerConfig.CallLifiAPI
	if err := Post(ctx, url, request, &resp); err != nil {
		return LifiBridgeResponse{}, "", err
	}
	return resp.LifiResponse, resp.Status, nil
}
