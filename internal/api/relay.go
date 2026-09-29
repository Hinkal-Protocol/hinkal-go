package api

import (
	"context"
	"errors"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type idleRelayRequest struct {
	ChainID       int  `json:"chainId"`
	MarkAsPending bool `json:"markAsPending"`
}

type idleRelayResponse struct {
	Relay string `json:"relay"`
}

func GetIdleRelay(ctx context.Context, chainID int, markAsPending bool) (string, error) {
	body := idleRelayRequest{ChainID: chainID, MarkAsPending: markAsPending}
	var resp idleRelayResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.RelayerConfig.GetIdleRelay, body, &resp); err != nil {
		return "", err
	}
	return resp.Relay, nil
}

type solanaRelayAddressRequest struct {
	ChainID int `json:"chainId"`
}

type solanaRelayAddressResponse struct {
	Status string `json:"status"`
	Relay  string `json:"relay,omitempty"`
	Error  string `json:"error,omitempty"`
}

// GetSolanaRelayAddress is henclave's Solana relay pubkey - a plain lookup, no signing capability revealed.
func GetSolanaRelayAddress(ctx context.Context, chainID int) (string, error) {
	body := solanaRelayAddressRequest{ChainID: chainID}
	var resp solanaRelayAddressResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.RelayerConfig.GetSolanaRelayAddress, body, &resp); err != nil {
		return "", err
	}
	if resp.Status != "success" || resp.Relay == "" {
		if resp.Error != "" {
			return "", errors.New(resp.Error)
		}
		return "", errors.New("no Solana relayer available")
	}
	return resp.Relay, nil
}
