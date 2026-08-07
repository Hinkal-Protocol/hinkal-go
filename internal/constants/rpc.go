package constants

import "fmt"

// FetchRPCURL returns a chain's fetch RPC URL from EthereumNetworkRegistry — the single source of
// truth for RPC endpoints.
func FetchRPCURL(chainID int) (string, error) {
	net, ok := EthereumNetworkRegistry[chainID]
	if !ok || net.FetchRPCURL == "" {
		return "", fmt.Errorf("no fetch rpc url configured for chain %d", chainID)
	}
	return net.FetchRPCURL, nil
}

// TronWalletRPCURL returns the HTTPS Tron full-node wallet API host for a chain, mirroring the TS
// getTronWalletRpcUrl. Used for the /wallet/* endpoints that the fetch RPC may not proxy.
func TronWalletRPCURL(chainID int) (string, error) {
	switch chainID {
	case ChainIDs.TronNile:
		return "https://nile.trongrid.io", nil
	case ChainIDs.TronMainnet:
		return "https://api.trongrid.io", nil
	default:
		return "", fmt.Errorf("unsupported Tron chain for wallet RPC: %d", chainID)
	}
}
