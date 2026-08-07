package bridging

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
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

type PendingEnclaveUtxo struct {
	ChainID            int
	SenderAddress      string
	RecipientAddress   string
	ClaimableSignature string
	Utxo               *utxo.Utxo
}

type PrivateBridgeResult struct {
	SourceTxHash       string
	DestTxHash         string
	PendingEnclaveUtxo *PendingEnclaveUtxo
}
