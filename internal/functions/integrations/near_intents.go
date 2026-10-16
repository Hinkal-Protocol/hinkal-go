package integrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func normalizeAssetAddress(chainID int, address string) string {
	if address == "" || !constants.IsTronLike(chainID) {
		return address
	}
	hexAddress, err := utils.AddressToHexFormat(address)
	if err != nil {
		return address
	}
	return hexAddress
}

// GetErc20TokenForNearAsset is the reverse of resolveNearAssetId: map a NEAR Intents token back to the in-app ERC20Token so a
// bridge refund sitting in the user's Intents account can be displayed and claimed like any other
// balance. Returns undefined for assets on chains/tokens the app does not support.
func GetErc20TokenForNearAsset(ctx context.Context, token types.NearIntentsToken) (*types.ERC20Token, error) {
	chainID, ok := constants.NearIntentsChainIDByBlockchain[token.Blockchain]
	if !ok {
		return nil, nil
	}
	contractAddress := normalizeAssetAddress(chainID, token.ContractAddress)
	if contractAddress != "" {
		found, err := web3.GetErc20TokenFromAPI(ctx, chainID, contractAddress)
		if err != nil {
			return nil, err
		}
		if found != nil {
			return found, nil
		}
	}
	bySymbol, err := api.GetTokensBySymbols(ctx, chainID, []string{token.Symbol})
	if err != nil {
		return nil, err
	}
	if len(bySymbol) == 0 || bySymbol[0] == nil {
		return nil, nil
	}
	return bySymbol[0], nil
}

func ResolveNearAssetID(ctx context.Context, chainID int, tokenAddress string) (string, error) {
	blockchain, ok := constants.NearIntentsBlockchain(chainID)
	if !ok {
		return "", fmt.Errorf("NEAR Intents does not support chain %d", chainID)
	}

	tokens, err := api.GetNearIntentsTokens(ctx)
	if err != nil {
		return "", err
	}

	isNative := constants.IsNativePlaceholderAddress(tokenAddress)
	for _, token := range tokens {
		if token.Blockchain != blockchain {
			continue
		}
		if isNative {
			if token.ContractAddress == "" {
				return token.AssetID, nil
			}
			continue
		}
		if token.ContractAddress != "" && strings.EqualFold(normalizeAssetAddress(chainID, token.ContractAddress), tokenAddress) {
			return token.AssetID, nil
		}
	}

	return "", fmt.Errorf("no NEAR Intents asset for token %s on chain %d", tokenAddress, chainID)
}
