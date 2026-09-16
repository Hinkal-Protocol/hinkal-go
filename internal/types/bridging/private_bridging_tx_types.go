package bridging

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type PrivateBridgeRecipient struct {
	RecipientInfo       string
	RecipientEthAddress string
	ClaimableNonce      *big.Int
}

func (r PrivateBridgeRecipient) IsClaimable() bool {
	return r.RecipientInfo == ""
}

type PrivateRecipientInfo struct {
	RecipientInfo string
	Amount        *big.Int
	Token         types.ERC20Token
}

type TokenChange struct {
	Token  types.ERC20Token
	Amount *big.Int
}

type PrivateBridgeOutResult struct {
	TempSubAccount types.TemporarySubAccount
	SourceTxHash   string
	ExpectedAmount *big.Int
}

type PrivateBridgeResult struct {
	SourceTxHash           string
	DestTxHash             string
	DestinationTokenAmount string
}
