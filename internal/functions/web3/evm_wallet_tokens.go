package web3

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func dedupeAddressesPreservingOrder(addresses []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if seen[address] {
			continue
		}
		seen[address] = true
		unique = append(unique, address)
	}
	return unique
}

// getEvmWalletTokenAddressesLocally mirrors the TS getEvmWalletTokenAddressesLocally
func getEvmWalletTokenAddressesLocally(ctx context.Context, chainID int, walletAddress string) ([]string, error) {
	nativeBalance, err := GetPublicBalanceByAddress(ctx, chainID, types.ERC20Token{Erc20TokenAddress: constants.ZeroAddress}, walletAddress)
	if err != nil {
		return nil, err
	}
	registryTokens, err := GetErc20TokensForChainAPI(ctx, chainID)
	if err != nil {
		return nil, err
	}

	heldTokens := make([]string, 0, len(registryTokens)+1)
	for _, token := range registryTokens {
		if token.Is2022Program {
			continue
		}
		balance, err := GetPublicBalanceByAddress(ctx, chainID, token, walletAddress)
		if err != nil || balance == nil || balance.Sign() <= 0 {
			continue
		}
		heldTokens = append(heldTokens, token.Erc20TokenAddress)
	}
	if nativeBalance != nil && nativeBalance.Sign() > 0 {
		heldTokens = append(heldTokens, constants.ZeroAddress)
	}

	return dedupeAddressesPreservingOrder(heldTokens), nil
}

// GetEvmWalletTokenAddresses mirrors the TS getEvmWalletTokenAddresses
func GetEvmWalletTokenAddresses(ctx context.Context, chainID int, walletAddress string) ([]string, error) {
	if constants.IsTempo(chainID) {
		return getEvmWalletTokenAddressesLocally(ctx, chainID, walletAddress)
	}

	balances, err := api.GetEvmTokenBalances(ctx, chainID, walletAddress)
	if err != nil {
		return nil, err
	}
	nativeBalance, err := GetPublicBalanceByAddress(ctx, chainID, types.ERC20Token{Erc20TokenAddress: constants.ZeroAddress}, walletAddress)
	if err != nil {
		return nil, err
	}
	if nativeBalance != nil && nativeBalance.Sign() > 0 {
		// Arc's gas token is USDC under the hood - representing it at zeroAddress here
		// too would show up as a second, duplicate USDC balance.
		nativeAddress := constants.ZeroAddress
		if constants.IsArcMainnet(chainID) {
			nativeAddress = constants.ArcMainnetUSDCAddress
		} else if constants.IsArcTestnet(chainID) {
			nativeAddress = constants.ArcTestnetUSDCAddress
		}
		balances = append(balances, api.EvmTokenBalanceEntry{ContractAddress: nativeAddress, TokenBalance: nativeBalance.String()})
	}

	addresses := make([]string, 0, len(balances))
	for _, balance := range balances {
		addresses = append(addresses, balance.ContractAddress)
	}
	return dedupeAddressesPreservingOrder(addresses), nil
}
