package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type UpdateDepositAndWithdrawStatusRequestBody struct {
	ID                    string                        `json:"id,omitempty"`
	HashedEthereumAddress string                        `json:"hashedEthereumAddress"`
	ChainID               int                           `json:"chainId"`
	Phase                 types.DepositAndWithdrawPhase `json:"phase"`
	DepositTxHash         string                        `json:"depositTxHash,omitempty"`
	ScheduleID            string                        `json:"scheduleId,omitempty"`
}

type DepositAndWithdrawStatusResponse struct {
	Status        string                        `json:"status"`
	ID            string                        `json:"id,omitempty"`
	Message       string                        `json:"message,omitempty"`
	Phase         types.DepositAndWithdrawPhase `json:"phase,omitempty"`
	DepositTxHash string                        `json:"depositTxHash,omitempty"`
	ScheduleID    string                        `json:"scheduleId,omitempty"`
	UpdatedAt     string                        `json:"updatedAt,omitempty"`
}

func UpdateDepositAndWithdrawStatus(ctx context.Context, body UpdateDepositAndWithdrawStatusRequestBody) (DepositAndWithdrawStatusResponse, error) {
	var resp DepositAndWithdrawStatusResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.UpdateDepositAndWithdrawStatus, body, &resp); err != nil {
		return DepositAndWithdrawStatusResponse{}, err
	}
	return resp, nil
}

// SafeUpdateDepositAndWithdrawStatus mirrors the TS SDK helper:
// best-effort status tracking after an irreversible deposit/schedule step has started.
// Use UpdateDepositAndWithdrawStatus for the initial before-deposit status creation,
// where failure must stop before user funds move on chain.
func SafeUpdateDepositAndWithdrawStatus(ctx context.Context, body UpdateDepositAndWithdrawStatusRequestBody) *DepositAndWithdrawStatusResponse {
	if body.ID == "" {
		return nil
	}
	resp, err := UpdateDepositAndWithdrawStatus(ctx, body)
	if err != nil {
		return nil
	}
	return &resp
}
