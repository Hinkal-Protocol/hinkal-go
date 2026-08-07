package types

import "github.com/Hinkal-Protocol/hinkal-go/internal/types"

type TokenBalanceJSON struct {
	Token     types.ERC20Token `json:"token"`
	Balance   string           `json:"balance"`
	Timestamp string           `json:"timestamp"`
}

type TxRequestJSON struct {
	To       string `json:"to"`
	Data     string `json:"data"`
	Value    string `json:"value"`
	GasLimit int64  `json:"gasLimit"`
}

type TxResultJSON struct {
	TxRequest TxRequestJSON `json:"txRequest"`
	Hash      string        `json:"hash"`
}
