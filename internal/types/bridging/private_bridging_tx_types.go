package bridging

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type PrivateRecipientInfo struct {
	RecipientInfo string
	Amount        *big.Int
	Token         types.ERC20Token
}

type TokenChange struct {
	Token  types.ERC20Token
	Amount *big.Int
}

type PrivateBridgeResult struct {
	SourceTxHash           string
	DestinationTokenAmount string
}
