package codec

import (
	"encoding/hex"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func EncodeTokenBalances(balances []types.TokenBalance) (string, error) {
	out := make([]mobiletypes.TokenBalanceJSON, 0, len(balances))
	for _, b := range balances {
		out = append(out, mobiletypes.TokenBalanceJSON{
			Token:     b.Token,
			Balance:   EncodeBig(b.Balance),
			Timestamp: b.Timestamp,
		})
	}
	return JSONString(out)
}

func encodeTxRequest(req types.TransactionRequest) mobiletypes.TxRequestJSON {
	data := ""
	if len(req.Data) > 0 {
		data = "0x" + hex.EncodeToString(req.Data)
	}
	return mobiletypes.TxRequestJSON{
		To:       req.To,
		Data:     data,
		Value:    EncodeBig(req.Value),
		GasLimit: int64(req.GasLimit),
	}
}

func EncodeTxResult(req types.TransactionRequest, hash string) (string, error) {
	return JSONString(mobiletypes.TxResultJSON{TxRequest: encodeTxRequest(req), Hash: hash})
}
