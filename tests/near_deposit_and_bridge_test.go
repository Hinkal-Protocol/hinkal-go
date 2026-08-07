package tests

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func nearTokenAddressEqual(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	if strings.HasPrefix(a, "0x") || strings.HasPrefix(b, "0x") {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func nearAssetID(t *testing.T, tokens []types.NearIntentsToken, chainID int, symbol, contractAddress string) string {
	t.Helper()
	blockchain, ok := constants.NearIntentsBlockchain(chainID)
	if !ok {
		t.Fatalf("no NEAR Intents blockchain mapping for chain %d", chainID)
	}
	for _, token := range tokens {
		if token.Blockchain != blockchain || !strings.EqualFold(token.Symbol, symbol) {
			continue
		}
		if contractAddress == "" || nearTokenAddressEqual(token.ContractAddress, contractAddress) {
			return token.AssetID
		}
	}
	t.Fatalf("no NEAR Intents asset for %s token %s on chain %d", symbol, contractAddress, chainID)
	return ""
}

func nearBridgeAmount(t *testing.T) *big.Int {
	t.Helper()
	amount := big.NewInt(200_000) // 0.2 USDC (6 decimals)
	if raw := os.Getenv("HINKAL_NEAR_BRIDGE_AMOUNT"); raw != "" {
		parsed, ok := new(big.Int).SetString(raw, 10)
		if !ok {
			t.Fatalf("bad HINKAL_NEAR_BRIDGE_AMOUNT: %q", raw)
		}
		amount = parsed
	}
	return amount
}

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 HINKAL_NEAR_DESTINATION_ADDRESS=0x... go test ./tests/... -run TestNearDepositAndBridgeSolanaToEvm_Live -v
// Solana USDC -> the EVM chain given by HINKAL_NEAR_DESTINATION_CHAIN / _DESTINATION_TOKEN.
func TestNearDepositAndBridgeSolanaToEvm_Live(t *testing.T) {
	requireLive(t)
	destinationRecipient := os.Getenv("HINKAL_NEAR_DESTINATION_ADDRESS")
	if destinationRecipient == "" {
		t.Skip("set HINKAL_NEAR_DESTINATION_ADDRESS to the destination-chain recipient address")
	}
	if !common.IsHexAddress(destinationRecipient) {
		t.Fatalf("HINKAL_NEAR_DESTINATION_ADDRESS is not an EVM address: %q", destinationRecipient)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Second)
	defer cancel()

	sourceChainID := constants.CurrentSolanaChainID
	destinationChainID := envChainID(t, "HINKAL_NEAR_DESTINATION_CHAIN", 0)
	destinationToken := os.Getenv("HINKAL_NEAR_DESTINATION_TOKEN")
	if destinationChainID == 0 || destinationToken == "" {
		t.Skip("set HINKAL_NEAR_DESTINATION_CHAIN and HINKAL_NEAR_DESTINATION_TOKEN to run the NEAR bridge test")
	}
	originAsset := os.Getenv("HINKAL_NEAR_ORIGIN_ASSET")
	destinationAsset := os.Getenv("HINKAL_NEAR_DESTINATION_ASSET")
	if originAsset == "" || destinationAsset == "" {
		tokens, err := api.GetNearIntentsTokens(ctx)
		if err != nil {
			t.Fatalf("near intents tokens: %v", err)
		}
		if originAsset == "" {
			originAsset = nearAssetID(t, tokens, sourceChainID, "USDC", solanaMainnetUSDC)
		}
		if destinationAsset == "" {
			destinationAsset = nearAssetID(t, tokens, destinationChainID, "USDC", destinationToken)
		}
	}

	h := newLiveSolanaHinkal(t, ctx)
	amount := nearBridgeAmount(t)
	result, err := h.NearDepositAndBridge(
		ctx,
		sourceChainID,
		solanaMainnetUSDC,
		[]*big.Int{amount},
		[]string{destinationRecipient},
		types.NearBridgeParams{
			OriginAsset:        originAsset,
			DestinationChainID: destinationChainID,
			DestinationAsset:   destinationAsset,
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("near deposit and bridge: %v", err)
	}
	if result.DepositTxHash == "" {
		t.Fatalf("deposit tx hash is empty")
	}
	if len(result.Legs) != 1 {
		t.Fatalf("legs length = %d, want 1", len(result.Legs))
	}
	leg := result.Legs[0]
	if leg.DepositAddress == "" {
		t.Fatalf("deposit address is empty")
	}
	if leg.Quote.AmountOut == "" {
		t.Fatalf("quote amountOut is empty")
	}
	t.Logf("near bridge: deposit=%s bridgeDeposit=%s in=%s out=%s", result.DepositTxHash, leg.DepositAddress, amount, leg.Quote.AmountOut)
}
