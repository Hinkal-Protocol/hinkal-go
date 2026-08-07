package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func SimulateVolatileTokenTransfer(ctx context.Context, chainID int, tokenTransfers []types.VolatileTokenChange) ([]types.VolatileTokenTransferResult, error) {
	body := map[string]any{
		"chainId":        chainID,
		"tokenTransfers": tokenTransfers,
	}
	var resp []types.VolatileTokenTransferResult
	if err := Post(ctx, constants.GetServerURL()+constants.ServerConfig.SimulateVolatileTokenTransfer, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
