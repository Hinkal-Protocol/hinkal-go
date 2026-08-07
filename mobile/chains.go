package mobile

import (
	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func SupportedChainsJSON() (string, error) {
	return codec.JSONString(constants.EVMChainIDs)
}

func AllSupportedChainsJSON() (string, error) {
	return codec.JSONString(constants.HinkalSupportedChains)
}

func TronChainID() int64 {
	return int64(constants.CurrentTronChainID())
}

func SolanaChainID() int64 {
	return int64(constants.CurrentSolanaChainID)
}

func IsEvmChain(chainID int64) bool {
	return constants.IsEvmChain(int(chainID))
}

func IsTronChain(chainID int64) bool {
	return constants.IsTronLike(int(chainID))
}

func IsSolanaChain(chainID int64) bool {
	return constants.IsSolanaLike(int(chainID))
}

func SolanaNativeTokenAddress() string {
	return constants.SolanaNativeAddress
}

func IsNearIntentsBridgeChain(chainID int64) bool {
	return constants.UsesNearIntentsBridge(int(chainID))
}

func IsBridgeSupportedChain(chainID int64) bool {
	return constants.IsBridgeSupportedChain(int(chainID))
}

func IsBridgeDestinationChain(chainID int64) bool {
	return constants.IsBridgeDestinationChain(int(chainID))
}

func BridgeDestinationChainsJSON(sourceChainID int64) (string, error) {
	if sourceChainID == 0 {
		return codec.JSONString(constants.GetBridgeDestinationChains())
	}
	return codec.JSONString(constants.GetBridgeDestinationChains(int(sourceChainID)))
}

func IsHinkalSupportedChain(chainID int64) bool {
	return constants.IsHinkalSupportedChain(int(chainID))
}

func IsBridgeDestinationOnlyChain(chainID int64) bool {
	return constants.IsBridgeDestinationOnlyChain(int(chainID))
}

func IsOptimismLikeChain(chainID int64) bool {
	return constants.IsOptimismLike(int(chainID))
}

func IsSepoliaTestnetChain(chainID int64) bool {
	return constants.IsSepoliaTestnet(int(chainID))
}

func IsTempoChain(chainID int64) bool {
	return constants.IsTempo(int(chainID))
}

func WalletSupportedChainsJSON() (string, error) {
	return codec.JSONString(constants.WalletSupportedChains)
}

func BridgeSupportedChainsJSON() (string, error) {
	return codec.JSONString(constants.BridgeSupportedChains)
}

func BridgeDestinationOnlyChainsJSON() (string, error) {
	return codec.JSONString(constants.BridgeDestinationOnlyChains)
}

func TronChainIDsJSON() (string, error) {
	return codec.JSONString(constants.TronChainIDs)
}

func SolanaChainIDsJSON() (string, error) {
	return codec.JSONString(constants.SolanaChainIDs)
}

func HinkalWrapperAddress(chainID int64) (string, error) {
	return constants.HinkalWrapperAddress(int(chainID))
}

func ChainIDsJSON() (string, error) {
	return codec.JSONString(constants.ChainIDs)
}

func NetworkRegistryJSON() (string, error) {
	return codec.JSONString(constants.EthereumNetworkRegistry)
}

func NetworkJSON(chainID int64) (string, error) {
	network, ok := constants.EthereumNetworkRegistry[int(chainID)]
	if !ok {
		return "", mobileerrors.UnknownChain(chainID)
	}
	return codec.JSONString(network)
}
