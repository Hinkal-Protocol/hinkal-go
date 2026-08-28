package constants

import "fmt"

// RPCURL returns a chain's public RPC URL from EthereumNetworkRegistry — the single source of
// truth for RPC endpoints.
func RPCURL(chainID int) (string, error) {
	net, ok := EthereumNetworkRegistry[chainID]
	if !ok || net.RPCURL == "" {
		return "", fmt.Errorf("no rpc url configured for chain %d", chainID)
	}
	return net.RPCURL, nil
}

// WithTronJSONRPCSuffix appends the JSON-RPC path to a Tron chain's base RPC URL: the base host
// serves /wallet/* REST, JSON-RPC (eth_call etc.) lives at /jsonrpc. No-op for non-Tron chains.
func WithTronJSONRPCSuffix(chainID int, rpcURL string) string {
	if !IsTronLike(chainID) {
		return rpcURL
	}
	return rpcURL + "/jsonrpc"
}

// TronJSONRPCURL returns a Tron chain's JSON-RPC endpoint, for callers that build the JSON-RPC
// request by hand instead of going through api.DialEVMWithFallback.
func TronJSONRPCURL(chainID int) (string, error) {
	rpcURL, err := RPCURL(chainID)
	if err != nil {
		return "", err
	}
	return WithTronJSONRPCSuffix(chainID, rpcURL), nil
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
