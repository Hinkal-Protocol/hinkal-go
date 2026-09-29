package pretransaction

import (
	"math/big"

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
	return nil
}
