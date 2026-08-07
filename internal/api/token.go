package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type getTokensInfoRequest struct {
	ChainID             int      `json:"chainId"`
	TokenAddresses      []string `json:"tokenAddresses"`
	WaitForOffsetUpdate bool     `json:"waitForOffsetUpdate"`
}

func GetTokensInfo(ctx context.Context, chainID int, tokenAddresses []string, waitForOffsetUpdate bool) ([]*types.ERC20Token, error) {
	if len(tokenAddresses) == 0 {
		return nil, nil
	}
	var resp []*types.ERC20Token
	body := getTokensInfoRequest{
		ChainID:             chainID,
		TokenAddresses:      tokenAddresses,
		WaitForOffsetUpdate: waitForOffsetUpdate,
	}
	if err := Post(ctx, constants.GetServerURL()+constants.ServerConfig.GetTokensInfo, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

type getTokensForChainRequest struct {
	ChainID int `json:"chainId"`
}

func GetTokensForChain(ctx context.Context, chainID int) ([]types.ERC20Token, error) {
	var resp []*types.ERC20Token
	body := getTokensForChainRequest{ChainID: chainID}
	if err := Post(ctx, constants.GetServerURL()+constants.ServerConfig.GetTokensForChain, body, &resp); err != nil {
		return nil, err
	}
	tokens := make([]types.ERC20Token, 0, len(resp))
	for _, token := range resp {
		if token != nil {
			tokens = append(tokens, *token)
		}
	}
	return tokens, nil
}
