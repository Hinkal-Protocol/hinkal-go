package api

import (
	"context"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
)

// evmAlwaysProxyMethods always go through the proxy, skipping the public RPC.
var evmAlwaysProxyMethods = map[string]bool{
	"eth_getTransactionReceipt": true,
	"eth_getLogs":               true,
}

// DialEVMWithFallback dials publicURL for chainID with a fallback-aware HTTP client.
func DialEVMWithFallback(chainID int, publicURL string) (*ethrpc.Client, error) {
	dialURL := constants.WithTronJSONRPCSuffix(chainID, publicURL)

	if constants.IsTempo(chainID) {
		return ethrpc.DialOptions(context.Background(), dialURL)
	}
	httpClient := NewFallbackHTTPClient(60*time.Second, evmAlwaysProxyMethods, func(routePath string) string {
		return constants.GetServerURL() + constants.ServerConfig.EvmRpcProxy(chainID, routePath)
	})
	return ethrpc.DialOptions(context.Background(), dialURL, ethrpc.WithHTTPClient(httpClient))
}

// DialEthClientWithFallback is DialEVMWithFallback wrapped in an *ethclient.Client
func DialEthClientWithFallback(chainID int, publicURL string) (*ethclient.Client, error) {
	rpcClient, err := DialEVMWithFallback(chainID, publicURL)
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(rpcClient), nil
}

// solanaAlwaysProxyMethods always go through the proxy, skipping the public RPC.
var solanaAlwaysProxyMethods = map[string]bool{
	"getLatestBlockhash":      true,
	"getBlockHeight":          true,
	"getTokenAccountsByOwner": true,
}

// SolanaFallbackHTTPClient returns a fallback-aware HTTP client for the Solana JSON-RPC client.
func SolanaFallbackHTTPClient() *http.Client {
	return NewFallbackHTTPClient(5*time.Minute, solanaAlwaysProxyMethods, func(routePath string) string {
		return constants.GetServerURL() + constants.ServerConfig.SolanaRpcProxy(routePath)
	})
}

func NewSolanaClientWithFallback(url string) *solana.Client {
	return solana.NewClientWithHTTPClient(url, SolanaFallbackHTTPClient())
}
