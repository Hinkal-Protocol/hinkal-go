package integrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
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
