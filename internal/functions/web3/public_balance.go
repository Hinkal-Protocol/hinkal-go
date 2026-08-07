package web3

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

func GetPublicBalanceByAddress(ctx context.Context, chainID int, tokenAddress, address string) (*big.Int, error) {
	client, err := fetchClient(chainID)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(tokenAddress, constants.ZeroAddress) {
		return client.BalanceAt(ctx, common.HexToAddress(address), nil)
	}
	return Erc20BalanceOf(ctx, client, common.HexToAddress(tokenAddress), common.HexToAddress(address))
}
