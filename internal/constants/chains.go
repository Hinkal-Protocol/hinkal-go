// mirrors chains.constants.ts
package constants

import (
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type chainIDs struct {
	Polygon        int
	ArbMainnet     int
	EthMainnet     int
	Optimism       int
	Base           int
	Localhost      int
	ArcTestnet     int
	SepoliaTestnet int
	SolanaMainnet  int
	SolanaLocalnet int
	TronNile       int
	TronLocalnet   int
	TronMainnet    int
	Tempo          int
	BNBMainnet     int

	// Bridge-destination-only chains (no Hinkal contracts deployed; reachable only as LiFi bridge targets).
	Avalanche int
	Cronos    int
	Monad     int
	Plasma    int
	Ink       int
	HyperEVM  int
}

var ChainIDs = chainIDs{
	Polygon:        137,
	ArbMainnet:     42161,
	EthMainnet:     1,
	Optimism:       10,
	Base:           8453,
	Localhost:      31337,
	ArcTestnet:     5042002,
	SepoliaTestnet: 11155111,
	SolanaMainnet:  501,
	SolanaLocalnet: 102,
	TronNile:       3448148188,
	TronLocalnet:   103,
	TronMainnet:    728126428,
	Tempo:          4217,
	BNBMainnet:     56,

	// Bridge-destination-only chains (no Hinkal contracts deployed; reachable only as LiFi bridge targets).
	Avalanche: 43114,
	Cronos:    25,
	Monad:     143,
	Plasma:    9745,
	Ink:       57073,
	HyperEVM:  999,
}

const SolanaChainIDStr = "4sGjMW1sUnHzSxGspuhpqLDx6wiyjNtZ"

var LocalhostNetwork = ChainIDs.EthMainnet

func IsLocalNetwork(chainID int) bool {
	return chainID == ChainIDs.Localhost ||
		chainID == ChainIDs.TronLocalnet ||
		chainID == ChainIDs.SolanaLocalnet
}

func GetNonLocalhostChainID(chainID int) int {
	if chainID != 0 {
		if IsLocalNetwork(chainID) {
			return LocalhostNetwork
		}
		return chainID
	}
	return LocalhostNetwork
}

// RPCURL below is public RPC only; internal/api's fallback transport proxies through Hinkal's backend when it fails.

var EthereumNetworkRegistry = map[int]types.EthereumNetwork{
	ChainIDs.EthMainnet: {
		Name:        "Ethereum",
		ChainID:     ChainIDs.EthMainnet,
		RPCURL:      "https://ethereum-rpc.publicnode.com",
		Supported:   true,
		Priority:    1,
		MaxPageSize: 900000,
	},
	ChainIDs.ArbMainnet: {
		Name:        "Arbitrum",
		ChainID:     ChainIDs.ArbMainnet,
		RPCURL:      "https://arbitrum-one-rpc.publicnode.com",
		Supported:   true,
		Priority:    2,
		MaxPageSize: 500000,
	},
	ChainIDs.Optimism: {
		Name:        "Optimism",
		ChainID:     ChainIDs.Optimism,
		RPCURL:      "https://optimism-rpc.publicnode.com",
		Supported:   true,
		Priority:    3,
		MaxPageSize: 900000,
	},
	ChainIDs.Polygon: {
		Name:        "Polygon",
		ChainID:     ChainIDs.Polygon,
		RPCURL:      "https://polygon-bor-rpc.publicnode.com",
		Supported:   true,
		Priority:    4,
		MaxPageSize: 900000,
	},
	ChainIDs.Base: {
		Name:        "Base",
		ChainID:     ChainIDs.Base,
		RPCURL:      "https://base-rpc.publicnode.com",
		Supported:   true,
		Priority:    7,
		MaxPageSize: 500000,
	},
	ChainIDs.ArcTestnet: {
		Name:        "Arc Testnet",
		ChainID:     ChainIDs.ArcTestnet,
		RPCURL:      "https://rpc.testnet.arc.network",
		Supported:   true,
		Priority:    8,
		MaxPageSize: 9999,
	},
	ChainIDs.SolanaMainnet: {
		Name:      "Solana",
		ChainID:   ChainIDs.SolanaMainnet,
		RPCURL:    "https://solana-rpc.publicnode.com",
		Supported: true,
		Priority:  8,
	},
	ChainIDs.SolanaLocalnet: {
		Name:      "Solana Localnet",
		ChainID:   ChainIDs.SolanaLocalnet,
		RPCURL:    "http://127.0.0.1:8899",
		Supported: true,
		Priority:  9,
	},
	ChainIDs.TronNile: {
		Name:        "Tron Nile",
		ChainID:     ChainIDs.TronNile,
		RPCURL:      "https://nile.trongrid.io",
		Supported:   true,
		Priority:    9,
		MaxPageSize: 500000,
	},
	ChainIDs.TronMainnet: {
		Name:        "Tron",
		ChainID:     ChainIDs.TronMainnet,
		RPCURL:      "https://tron-rpc.publicnode.com",
		Supported:   true,
		Priority:    10,
		MaxPageSize: 500000,
	},
	ChainIDs.SepoliaTestnet: {
		Name:        "Sepolia Testnet",
		ChainID:     ChainIDs.SepoliaTestnet,
		RPCURL:      "https://ethereum-sepolia-rpc.publicnode.com",
		Supported:   true,
		Priority:    11,
		MaxPageSize: 900000,
	},
	ChainIDs.Tempo: {
		Name:        "Tempo",
		ChainID:     ChainIDs.Tempo,
		RPCURL:      "https://rpc.tempo.xyz",
		Supported:   true,
		Priority:    12,
		MaxPageSize: 9999,
	},
	ChainIDs.BNBMainnet: {
		Name:        "BNB Chain",
		ChainID:     ChainIDs.BNBMainnet,
		RPCURL:      "https://bsc-rpc.publicnode.com",
		Supported:   true,
		Priority:    13,
		MaxPageSize: 900000,
	},

	// Bridge-destination-only chains: no Hinkal contracts, only valid as LiFi bridge targets in pay/dashboard.
	ChainIDs.Avalanche: {
		Name:      "Avalanche",
		ChainID:   ChainIDs.Avalanche,
		RPCURL:    "https://api.avax.network/ext/bc/C/rpc",
		Supported: false,
		Priority:  14,
	},
	ChainIDs.Cronos: {
		Name:      "Cronos",
		ChainID:   ChainIDs.Cronos,
		RPCURL:    "https://evm.cronos.org",
		Supported: false,
		Priority:  15,
	},
	ChainIDs.Monad: {
		Name:      "Monad",
		ChainID:   ChainIDs.Monad,
		RPCURL:    "https://rpc.monad.xyz",
		Supported: false,
		Priority:  16,
	},
	ChainIDs.Plasma: {
		Name:      "Plasma",
		ChainID:   ChainIDs.Plasma,
		RPCURL:    "https://rpc.plasma.to",
		Supported: false,
		Priority:  17,
	},
	ChainIDs.Ink: {
		Name:      "Ink",
		ChainID:   ChainIDs.Ink,
		RPCURL:    "https://rpc-gel.inkonchain.com",
		Supported: false,
		Priority:  18,
	},
	ChainIDs.HyperEVM: {
		Name:      "HyperEVM",
		ChainID:   ChainIDs.HyperEVM,
		RPCURL:    "https://rpc.hyperliquid.xyz/evm",
		Supported: false,
		Priority:  19,
	},
}

var TronChainIDs = []int{ChainIDs.TronNile, ChainIDs.TronMainnet, ChainIDs.TronLocalnet}

var SolanaChainIDs = []int{ChainIDs.SolanaMainnet, ChainIDs.SolanaLocalnet}

var HinkalSupportedChains = []int{
	ChainIDs.EthMainnet,
	ChainIDs.Optimism,
	ChainIDs.Base,
	ChainIDs.Polygon,
	ChainIDs.ArbMainnet,
	ChainIDs.BNBMainnet,
	ChainIDs.ArcTestnet,
	ChainIDs.SolanaMainnet,
	ChainIDs.SepoliaTestnet,
	ChainIDs.TronMainnet,
	ChainIDs.TronNile,
	ChainIDs.Tempo,
}

var EVMChainIDs = func() []int {
	var ids []int
	for _, id := range HinkalSupportedChains {
		if !IsSolanaLike(id) && !IsTronLike(id) {
			ids = append(ids, id)
		}
	}
	return ids
}()

var CurrentSolanaChainID = ChainIDs.SolanaMainnet

func CurrentTronChainID() int {
	if Mode != DeploymentModeProduction {
		return ChainIDs.TronNile
	}
	return ChainIDs.TronMainnet
}

var WalletSupportedChains = func() []int {
	var ids []int
	for _, id := range HinkalSupportedChains {
		if !IsTronLike(id) && !IsSepoliaTestnet(id) && !IsTempo(id) && !IsArcTestnet(id) {
			ids = append(ids, id)
		}
	}
	return ids
}()

// Chains that can act as a bridge source (Hinkal contracts deployed, user can hold a balance).
var BridgeSupportedChains = func() []int {
	var ids []int
	for _, id := range HinkalSupportedChains {
		if id != ChainIDs.ArcTestnet && !IsSepoliaTestnet(id) && (!IsTronLike(id) || id == ChainIDs.TronMainnet) {
			ids = append(ids, id)
		}
	}
	return ids
}()

// Chains we can bridge to via LiFi but where Hinkal has no contracts deployed.
// They are valid bridge destinations in pay/dashboard only, never a source/wallet/balance chain.
var BridgeDestinationOnlyChains = []int{
	ChainIDs.Avalanche,
	ChainIDs.Cronos,
	ChainIDs.Monad,
	ChainIDs.Plasma,
	ChainIDs.Ink,
	ChainIDs.HyperEVM,
}

var BridgeDestinationChains = append(copyChainIDs(BridgeSupportedChains), BridgeDestinationOnlyChains...)

func IsHinkalSupportedChain(chainID int) bool {
	return contains(HinkalSupportedChains, chainID)
}

func IsBridgeSupportedChain(chainID int) bool {
	return contains(BridgeSupportedChains, chainID)
}

func IsBridgeDestinationOnlyChain(chainID int) bool {
	return contains(BridgeDestinationOnlyChains, chainID)
}

func IsBridgeDestinationChain(chainID int) bool {
	return contains(BridgeDestinationChains, chainID)
}

func GetBridgeDestinationChains(sourceChainID ...int) []int {
	if len(sourceChainID) > 0 && UsesNearIntentsBridge(sourceChainID[0]) {
		var ids []int
		for _, chainID := range BridgeDestinationChains {
			if IsNearBridgeSupportedChain(chainID) {
				ids = append(ids, chainID)
			}
		}
		return ids
	}
	return copyChainIDs(BridgeDestinationChains)
}

var SaveDepths = map[int]uint64{
	ChainIDs.EthMainnet:     1000,
	ChainIDs.BNBMainnet:     1000,
	ChainIDs.Polygon:        4000,
	ChainIDs.ArbMainnet:     8000,
	ChainIDs.Optimism:       6000,
	ChainIDs.Base:           6000,
	ChainIDs.ArcTestnet:     8000,
	ChainIDs.SepoliaTestnet: 1000,
	ChainIDs.Tempo:          4000,
	ChainIDs.Localhost:      1,
	ChainIDs.SolanaMainnet:  1000,
	ChainIDs.SolanaLocalnet: 100,
	ChainIDs.TronNile:       1000,
	ChainIDs.TronMainnet:    1000,
	ChainIDs.TronLocalnet:   100,
}

var BlockReorgDepths = map[int]uint64{
	ChainIDs.EthMainnet:     300,
	ChainIDs.BNBMainnet:     1000,
	ChainIDs.Polygon:        1000,
	ChainIDs.ArbMainnet:     1000,
	ChainIDs.Optimism:       1000,
	ChainIDs.Base:           1000,
	ChainIDs.ArcTestnet:     30,
	ChainIDs.SepoliaTestnet: 300,
	ChainIDs.Tempo:          1000,
	ChainIDs.TronNile:       19,
	ChainIDs.TronMainnet:    19,
	ChainIDs.TronLocalnet:   1,
	ChainIDs.Localhost:      1,
}

func IsOptimismLike(chainID int) bool {
	return chainID == ChainIDs.Optimism || chainID == ChainIDs.Base
}

func IsSolanaLike(chainID int) bool {
	return contains(SolanaChainIDs, chainID)
}

func IsTronLike(chainID int) bool {
	return contains(TronChainIDs, chainID)
}

func IsEnclaveTxChain(chainID int) bool {
	return !IsSolanaLike(chainID)
}

func IsEvmChain(chainID int) bool {
	return contains(EVMChainIDs, chainID)
}

func UsesNearIntentsBridge(chainID int) bool {
	return IsSolanaLike(chainID) || IsTronLike(chainID)
}

func IsSepoliaTestnet(chainID int) bool {
	return chainID == ChainIDs.SepoliaTestnet
}

func IsTempo(chainID int) bool {
	return chainID == ChainIDs.Tempo
}

func IsArcTestnet(chainID int) bool {
	return chainID == ChainIDs.ArcTestnet
}

func GetSaveDepth(chainID int) (uint64, error) {
	if d, ok := SaveDepths[chainID]; ok {
		return d, nil
	}
	return 0, fmt.Errorf("no save depth configured for chain %d", chainID)
}

func GetReorgDepth(chainID int) (uint64, error) {
	if d, ok := BlockReorgDepths[chainID]; ok {
		return d, nil
	}
	return 0, fmt.Errorf("no reorg depth configured for chain %d", chainID)
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func copyChainIDs(ids []int) []int {
	return append([]int(nil), ids...)
}
