package mobile

import (
	tokens "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/tokens"
)

func GetTokensJSON(chainID int64) (string, error) {
	return tokens.GetTokensJSON(chainID)
}

func AmountToWei(chainID int64, tokenAddr, amount string) (string, error) {
	return tokens.AmountToWei(chainID, tokenAddr, amount)
}

func AmountFromWei(chainID int64, tokenAddr, amountWei string) (string, error) {
	return tokens.AmountFromWei(chainID, tokenAddr, amountWei)
}

func AmountWithPrecision(chainID int64, tokenAddr, amountWei string, precision int64) (string, error) {
	return tokens.AmountWithPrecision(chainID, tokenAddr, amountWei, precision)
}

func ResolveTokensJSON(chainID int64, tokenAddrsJSON string) (string, error) {
	return tokens.ResolveTokensJSON(chainID, tokenAddrsJSON)
}

func ResolveTokensLenientJSON(chainID int64, tokenAddrsJSON string) (string, error) {
	return tokens.ResolveTokensLenientJSON(chainID, tokenAddrsJSON)
}
