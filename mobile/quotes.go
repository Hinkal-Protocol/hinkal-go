package mobile

import (
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	quotes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/quotes"
)

const (
	ActionTransact = string(types.ExternalActionTransact)
	ActionLifi     = string(types.ExternalActionLifi)
	ActionOkx      = string(types.ExternalActionOkx)
	ActionEmporium = string(types.ExternalActionEmporium)
)

func GetSwapQuotesJSON(chainID64 int64, inAmount, inTokenAddr, outTokenAddr string) (string, error) {
	return quotes.GetSwapQuotesJSON(chainID64, inAmount, inTokenAddr, outTokenAddr)
}

func GetExternalSwapAddress(chainID64 int64, actionID string) (string, error) {
	return quotes.GetExternalSwapAddress(chainID64, actionID)
}

func GetOKXQuoteJSON(chainID64 int64, inAmount, inTokenAddr, outTokenAddr string, slippagePercentage float64) (string, error) {
	return quotes.GetOKXQuoteJSON(chainID64, inAmount, inTokenAddr, outTokenAddr, slippagePercentage)
}

func GetLifiBridgeQuoteJSON(
	sourceChainID, destinationChainID int64,
	sourceTokenAddr, destinationTokenAddr string,
	amount string,
	slippage float64,
	fromAddress, toAddress string,
) (string, error) {
	return quotes.GetLifiBridgeQuoteJSON(sourceChainID, destinationChainID, sourceTokenAddr, destinationTokenAddr, amount, slippage, fromAddress, toAddress)
}

func GetNearIntentsQuoteJSON(requestJSON string) (string, error) {
	return quotes.GetNearIntentsQuoteJSON(requestJSON)
}

func GetNearIntentsTokensJSON() (string, error) {
	return quotes.GetNearIntentsTokensJSON()
}

func NewTemporarySubAccountJSON(index int64) (string, error) {
	return quotes.NewTemporarySubAccountJSON(index)
}
