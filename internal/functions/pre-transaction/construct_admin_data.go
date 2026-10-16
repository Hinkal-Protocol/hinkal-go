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
	variableRate *big.Int,
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

	hashedOwner := ""
	if !isPrivateTransferOrWithdraw(action) {
		hashedOwner = utils.HashEthereumAddress(ethereumAddress)
	}

	variableRateOverride := ""
	if variableRate != nil && variableRate.Sign() > 0 {
		variableRateOverride = variableRate.String()
	}

	return &types.AdminDataType{
		HashedOwner:          hashedOwner,
		ChainID:              chainID,
		Timestamp:            time.Now().UnixMilli(),
		Action:               action,
		Erc20TokenAddresses:  erc20TokenAddresses,
		Erc20TokenAmount:     amounts,
		SwapPair:             swapPair,
		VariableRateOverride: variableRateOverride,
	}
}
