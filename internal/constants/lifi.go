package constants

import "fmt"

const LifiSolanaChainID = 1151111081099710

func GetLifiChainID(chainID int) int {
	switch chainID {
	case ChainIDs.SolanaMainnet, ChainIDs.SolanaLocalnet:
		return LifiSolanaChainID
	default:
		return chainID
	}
}

func GetLifiBridgeDestinationChains() []int {
	return copyChainIDs(BridgeDestinationChains)
}

func IsLifiBridgeDestinationChain(chainID int) bool {
	return IsBridgeDestinationChain(chainID)
}

func LifiRouterAddress(chainID int) (string, error) {
	switch chainID {
	case ChainIDs.EthMainnet,
		ChainIDs.Polygon,
		ChainIDs.Optimism,
		ChainIDs.ArbMainnet,
		ChainIDs.BNBMainnet,
		ChainIDs.Base:
		return "0x1231DEB6f5749EF6cE6943a275A1D3E7486F4EaE", nil
	case ChainIDs.Tempo:
		return "0x2cAcAE8e22418E65dcf7651c67aEbe6288EB8243", nil
	case ChainIDs.SolanaMainnet, ChainIDs.SolanaLocalnet:
		return "3i5JeuZuUxeKtVysUnwQNGerJP2bSMX9fTFfS4Nxe3Br", nil
	default:
		return "", fmt.Errorf("constants: Lifi router address not set for chain %d", chainID)
	}
}
