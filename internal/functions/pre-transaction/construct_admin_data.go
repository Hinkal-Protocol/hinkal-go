package pretransaction

import (
	"math/big"
	"strings"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func ConstructAdminData(
	action types.AdminTransactionType,
	chainID int,
	erc20TokenAddresses []string,
	amountChanges []*big.Int,
	ethereumAddress string,
	swapPairTokens []types.ERC20Token,
) *types.AdminDataType {
	if action == "" {
		return nil
	}

	var swapPair *types.SwapPair
	if len(swapPairTokens) >= 2 {
		swapPair = &types.SwapPair{
			TokenIn: types.SwapPairToken{
				Symbol:  swapPairTokens[0].Symbol,
				Address: strings.ToLower(swapPairTokens[0].Erc20TokenAddress),
			},
			TokenOut: types.SwapPairToken{
				Symbol:  swapPairTokens[1].Symbol,
				Address: strings.ToLower(swapPairTokens[1].Erc20TokenAddress),
			},
		}
	}

	amounts := make([]string, len(amountChanges))
	for i, amount := range amountChanges {
		amounts[i] = amount.String()
	}

	return &types.AdminDataType{
		HashedOwner:         utils.HashEthereumAddress(ethereumAddress),
		ChainID:             chainID,
		Timestamp:           time.Now().UnixMilli(),
		Action:              action,
		Erc20TokenAddresses: erc20TokenAddresses,
		Erc20TokenAmount:    amounts,
		SwapPair:            swapPair,
	}
}
