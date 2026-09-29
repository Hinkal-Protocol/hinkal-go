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
	case ChainIDs.Robinhood:
		return "0xB477751B76CF82d00a686A1232f5fCD772414Af3", nil
	case ChainIDs.ArcMainnet:
		return "0xA4072583658Fae592A3506A42431cb6316a8d40b", nil
	case ChainIDs.ArcTestnet:
		return "0xFf70F4A1d11995621854F3692acF286d8aCd04b2", nil
	case ChainIDs.SolanaMainnet, ChainIDs.SolanaLocalnet:
		return "3i5JeuZuUxeKtVysUnwQNGerJP2bSMX9fTFfS4Nxe3Br", nil
	default:
		return "", fmt.Errorf("constants: Lifi router address not set for chain %d", chainID)
	}
}
