package constants

const (
	NearBridgeQuoteDeadlineBufferMS int64 = 20 * 60 * 1000
	NearBridgeSlippageBPS                 = 100
	NearRPCURL                            = "https://rpc.mainnet.fastnear.com"
	NearRPCMaxRetryAttempts               = 12
	NearRPCRetryDelayMS                   = 500
	NearRPCRetryBackoff                   = 1.5
	NearIntentsContractID                 = "intents.near"
	NearIntentsReferral                   = "hinkal"
)

var NearRPCRetryStatuses = []int{500, 503, 408}

var NearIntentsBlockchainByChainID = map[int]string{
	ChainIDs.SolanaMainnet: "sol",
	ChainIDs.EthMainnet:    "eth",
	ChainIDs.Base:          "base",
	ChainIDs.ArbMainnet:    "arb",
	ChainIDs.Optimism:      "op",
	ChainIDs.Polygon:       "pol",
	ChainIDs.TronMainnet:   "tron",
	ChainIDs.BNBMainnet:    "bsc",
	ChainIDs.Avalanche:     "avax",
	ChainIDs.Monad:         "monad",
	ChainIDs.Plasma:        "plasma",
}

var NearIntentsChainIDByBlockchain = func() map[string]int {
	ids := make(map[string]int, len(NearIntentsBlockchainByChainID))
	for chainID, blockchain := range NearIntentsBlockchainByChainID {
		ids[blockchain] = chainID
	}
	return ids
}()

var NearUnsupportedBridgeDestinations = func() []int {
	var ids []int
	for _, chainID := range BridgeDestinationChains {
		if !IsNearBridgeSupportedChain(chainID) {
			ids = append(ids, chainID)
		}
	}
	return ids
}()

func NearIntentsBlockchain(chainID int) (string, bool) {
	blockchain, ok := NearIntentsBlockchainByChainID[chainID]
	return blockchain, ok
}

func IsNearBridgeSupportedChain(chainID int) bool {
	_, ok := NearIntentsBlockchain(chainID)
	return ok
}
