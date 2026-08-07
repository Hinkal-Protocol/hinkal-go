package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type ScheduledTransactionsNullifierIndexesRequest struct {
	HashedEthereumAddress string   `json:"hashedEthereumAddress"`
	Nullifiers            []string `json:"nullifiers"`
}

type ScheduledTransactionsNullifierIndexesResponse struct {
	Indexes []int `json:"indexes"`
}

func GetScheduledTransactionsNullifierIndexes(ctx context.Context, body ScheduledTransactionsNullifierIndexesRequest) (*ScheduledTransactionsNullifierIndexesResponse, error) {
	var resp ScheduledTransactionsNullifierIndexesResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.GetScheduledTransactionsNullifierIndexes, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func GetScheduledTransactionByID(ctx context.Context, scheduleID string) (types.ScheduledTransactionByIDResponse, error) {
	var resp types.ScheduledTransactionByIDResponse
	if err := Get(ctx, constants.GetRelayerURL()+constants.RelayerConfig.GetScheduledTransactionByID(scheduleID), &resp); err != nil {
		return types.ScheduledTransactionByIDResponse{}, err
	}
	return resp, nil
}
