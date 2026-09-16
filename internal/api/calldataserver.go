package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type emitTxPublicDataBody struct {
	AdminData *types.AdminDataType `json:"adminData"`
}

func EmitTxPublicData(ctx context.Context, adminData *types.AdminDataType) error {
	if adminData == nil {
		return nil
	}
	return Post(ctx, constants.GetDataServerURL()+constants.DataServerConfig.EmitTxPublicData, emitTxPublicDataBody{AdminData: adminData}, nil)
}

type GetTokenPricesResponse struct {
	Prices []float64 `json:"prices"`
}

func GetTokenPrices(ctx context.Context, chainID int, erc20Addresses []string) (*GetTokenPricesResponse, error) {
	body := map[string]any{
		"erc20Addresses": erc20Addresses,
		"chainId":        chainID,
	}
	var resp GetTokenPricesResponse
	if err := Post(ctx, constants.GetDataServerURL()+constants.DataServerConfig.GetTokenPrices, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
