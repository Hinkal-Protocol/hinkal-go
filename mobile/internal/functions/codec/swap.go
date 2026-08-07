package codec

import (
	"math/big"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func AppendSwapQuote(quotes []mobiletypes.SwapQuoteJSON, id types.ExternalActionID, amount *big.Int, data string) []mobiletypes.SwapQuoteJSON {
	if amount == nil || amount.Sign() <= 0 {
		return quotes
	}
	return append(quotes, mobiletypes.SwapQuoteJSON{ActionID: string(id), OutAmount: amount.String(), SwapData: data})
}

func EncodeEVMSwapPrices(price *integrations.EVMSwapPrice) (string, error) {
	quotes := make([]mobiletypes.SwapQuoteJSON, 0, 1)
	if price != nil {
		quotes = AppendSwapQuote(quotes, types.ExternalActionLifi, price.OutSwapAmountValue, price.LifiDataValue)
	}
	return JSONString(quotes)
}

func EncodeSolanaSwapPrices(price *integrations.SolanaSwapPrice) (string, error) {
	quotes := make([]mobiletypes.SwapQuoteJSON, 0, 1)
	if price != nil {
		quotes = AppendSwapQuote(quotes, types.ExternalActionOkx, price.OutSwapAmountValue, price.OKXDataValue)
	}
	return JSONString(quotes)
}
