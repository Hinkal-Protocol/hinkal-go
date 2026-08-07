package tokens

import (
	"context"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func GetTokensJSON(chainID int64) (string, error) {
	list, err := api.GetTokensForChain(context.Background(), int(chainID))
	if err != nil {
		return "", err
	}
	return codec.JSONString(list)
}

func AmountToWei(chainID int64, tokenAddr, amount string) (string, error) {
	if amount == "" {
		return "", mobileerrors.ErrEmptyAmount
	}
	token, err := web3.ResolveERC20TokenStrict(context.Background(), int(chainID), tokenAddr)
	if err != nil {
		return "", err
	}
	wei, err := web3.GetAmountInWei(token, amount)
	if err != nil {
		return "", err
	}
	return codec.EncodeBig(wei), nil
}

func AmountFromWei(chainID int64, tokenAddr, amountWei string) (string, error) {
	wei, err := codec.DecodeBig(amountWei)
	if err != nil {
		return "", err
	}
	token, err := web3.ResolveERC20TokenStrict(context.Background(), int(chainID), tokenAddr)
	if err != nil {
		return "", err
	}
	return web3.GetAmountInToken(token, wei), nil
}

func AmountWithPrecision(chainID int64, tokenAddr, amountWei string, precision int64) (string, error) {
	wei, err := codec.DecodeBig(amountWei)
	if err != nil {
		return "", err
	}
	token, err := web3.ResolveERC20TokenStrict(context.Background(), int(chainID), tokenAddr)
	if err != nil {
		return "", err
	}
	return web3.GetAmountWithPrecision(wei, token, int(precision))
}

func ResolveTokensJSON(chainID int64, tokenAddrsJSON string) (string, error) {
	addrs, err := codec.DecodeStrings(tokenAddrsJSON)
	if err != nil {
		return "", err
	}
	resolved, err := web3.ResolveERC20TokensStrict(context.Background(), int(chainID), addrs)
	if err != nil {
		return "", err
	}
	return codec.JSONString(resolved)
}

func ResolveTokensLenientJSON(chainID int64, tokenAddrsJSON string) (string, error) {
	addrs, err := codec.DecodeStrings(tokenAddrsJSON)
	if err != nil {
		return "", err
	}
	resolved, err := web3.ResolveERC20Tokens(context.Background(), int(chainID), addrs)
	if err != nil {
		return "", err
	}
	return codec.JSONString(resolved)
}
