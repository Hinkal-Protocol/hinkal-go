package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

func Monitor(ctx context.Context, address string) error {
	body := map[string]any{
		"address": address,
	}
	return Post(ctx, constants.GetServerURL()+constants.ServerConfig.Monitor, body, nil)
}

type RiskyAddress struct {
	Address       string `json:"address"`
	BlockedReason string `json:"blockedReason,omitempty"`
}

type MonitorBatchResponse struct {
	RiskyAddresses []RiskyAddress `json:"riskyAddresses"`
}

func MonitorBatch(ctx context.Context, addresses []string) (*MonitorBatchResponse, error) {
	body := map[string]any{
		"addresses": addresses,
	}
	var resp MonitorBatchResponse
	if err := Post(ctx, constants.GetServerURL()+constants.ServerConfig.MonitorBatch, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
