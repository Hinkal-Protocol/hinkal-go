package tests

import (
	"context"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
)

func TestChainFamilyHelpers(t *testing.T) {
	for _, chainID := range []int{constants.ChainIDs.Localhost, constants.ChainIDs.TronLocalnet, constants.ChainIDs.SolanaLocalnet} {
		if !constants.IsLocalNetwork(chainID) {
			t.Fatalf("IsLocalNetwork(%d) = false", chainID)
		}
	}

	for _, chainID := range []int{constants.ChainIDs.EthMainnet, constants.ChainIDs.TronMainnet, constants.ChainIDs.SolanaMainnet} {
		if constants.IsLocalNetwork(chainID) {
			t.Fatalf("IsLocalNetwork(%d) = true", chainID)
		}
	}

	for _, chainID := range []int{constants.ChainIDs.SolanaMainnet, constants.ChainIDs.SolanaLocalnet} {
		if !constants.IsSolanaLike(chainID) {
			t.Fatalf("IsSolanaLike(%d) = false", chainID)
		}
		if constants.IsTronLike(chainID) {
			t.Fatalf("IsTronLike(%d) = true", chainID)
		}
		if !constants.UsesNearIntentsBridge(chainID) {
			t.Fatalf("UsesNearIntentsBridge(%d) = false", chainID)
		}
	}

	for _, chainID := range []int{constants.ChainIDs.TronNile, constants.ChainIDs.TronMainnet, constants.ChainIDs.TronLocalnet} {
		if !constants.IsTronLike(chainID) {
			t.Fatalf("IsTronLike(%d) = false", chainID)
		}
		if constants.IsSolanaLike(chainID) {
			t.Fatalf("IsSolanaLike(%d) = true", chainID)
		}
		if !constants.UsesNearIntentsBridge(chainID) {
			t.Fatalf("UsesNearIntentsBridge(%d) = false", chainID)
		}
	}

	if constants.IsSolanaLike(constants.ChainIDs.Polygon) {
		t.Fatal("Polygon should not be Solana-like")
	}
	if constants.IsTronLike(constants.ChainIDs.Polygon) {
		t.Fatal("Polygon should not be Tron-like")
	}
	if constants.UsesNearIntentsBridge(constants.ChainIDs.Polygon) {
		t.Fatal("Polygon should not use the NEAR intents bridge")
	}
}

func TestBridgeDestinationChains(t *testing.T) {
	destinationOnlyChains := []int{
		constants.ChainIDs.Avalanche,
		constants.ChainIDs.Cronos,
		constants.ChainIDs.Monad,
		constants.ChainIDs.Plasma,
		constants.ChainIDs.Ink,
		constants.ChainIDs.HyperEVM,
	}

	for _, chainID := range destinationOnlyChains {
		if !constants.IsBridgeDestinationOnlyChain(chainID) {
			t.Fatalf("IsBridgeDestinationOnlyChain(%d) = false", chainID)
		}
		if !constants.IsBridgeDestinationChain(chainID) {
			t.Fatalf("IsBridgeDestinationChain(%d) = false", chainID)
		}
		if !constants.IsLifiBridgeDestinationChain(chainID) {
			t.Fatalf("IsLifiBridgeDestinationChain(%d) = false", chainID)
		}
		if constants.IsHinkalSupportedChain(chainID) {
			t.Fatalf("destination-only chain %d should not be Hinkal-supported", chainID)
		}
		if constants.IsBridgeSupportedChain(chainID) {
			t.Fatalf("destination-only chain %d should not be bridge-source-supported", chainID)
		}
	}

	for _, chainID := range []int{constants.ChainIDs.EthMainnet, constants.ChainIDs.Base, constants.ChainIDs.BNBMainnet, constants.ChainIDs.SolanaMainnet, constants.ChainIDs.TronMainnet, constants.ChainIDs.Tempo} {
		if !constants.IsBridgeSupportedChain(chainID) {
			t.Fatalf("IsBridgeSupportedChain(%d) = false", chainID)
		}
	}

	for _, chainID := range []int{constants.ChainIDs.ArcTestnet, constants.ChainIDs.SepoliaTestnet, constants.ChainIDs.TronNile} {
		if constants.IsBridgeSupportedChain(chainID) {
			t.Fatalf("IsBridgeSupportedChain(%d) = true", chainID)
		}
	}
}

func TestNearBridgeDestinationFiltering(t *testing.T) {
	fullDestinations := constants.GetBridgeDestinationChains()
	for _, chainID := range []int{constants.ChainIDs.Cronos, constants.ChainIDs.Ink, constants.ChainIDs.HyperEVM} {
		if !hasChain(fullDestinations, chainID) {
			t.Fatalf("full bridge destinations should include %d", chainID)
		}
	}

	nearDestinations := constants.GetBridgeDestinationChains(constants.ChainIDs.SolanaMainnet)
	for _, chainID := range []int{
		constants.ChainIDs.BNBMainnet,
		constants.ChainIDs.Avalanche,
		constants.ChainIDs.Monad,
		constants.ChainIDs.Plasma,
	} {
		if !constants.IsNearBridgeSupportedChain(chainID) {
			t.Fatalf("IsNearBridgeSupportedChain(%d) = false", chainID)
		}
		if !hasChain(nearDestinations, chainID) {
			t.Fatalf("NEAR bridge destinations should include %d", chainID)
		}
	}

	for _, chainID := range []int{constants.ChainIDs.Cronos, constants.ChainIDs.Ink, constants.ChainIDs.HyperEVM} {
		if constants.IsNearBridgeSupportedChain(chainID) {
			t.Fatalf("IsNearBridgeSupportedChain(%d) = true", chainID)
		}
		if hasChain(nearDestinations, chainID) {
			t.Fatalf("NEAR bridge destinations should not include %d", chainID)
		}
	}

	blockchain, ok := constants.NearIntentsBlockchain(constants.ChainIDs.BNBMainnet)
	if !ok || blockchain != "bsc" {
		t.Fatalf("NearIntentsBlockchain(BNB) = %q, %v; want bsc, true", blockchain, ok)
	}
}

func TestBridgeDestinationTokenRegistries(t *testing.T) {
	requireLive(t)
	for _, chainID := range []int{
		constants.ChainIDs.Avalanche,
		constants.ChainIDs.Cronos,
		constants.ChainIDs.Monad,
		constants.ChainIDs.Plasma,
		constants.ChainIDs.Ink,
		constants.ChainIDs.HyperEVM,
	} {
		tokens, err := web3.GetErc20TokensForChainAPI(context.Background(), chainID)
		if err != nil {
			t.Fatalf("GetErc20TokensForChainAPI(%d): %v", chainID, err)
		}
		if len(tokens) == 0 {
			t.Fatalf("GetErc20TokensForChainAPI(%d) returned no tokens", chainID)
		}
	}
}

func hasChain(chains []int, want int) bool {
	for _, chainID := range chains {
		if chainID == want {
			return true
		}
	}
	return false
}
