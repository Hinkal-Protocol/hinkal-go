package tests

import (
	"context"
	"encoding/hex"
	"os"
	"testing"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func temporarySubAccount(t *testing.T) types.TemporarySubAccount {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate temporary wallet: %v", err)
	}
	return types.TemporarySubAccount{
		Index:      int(time.Now().UnixNano() % 1_000_000_000),
		EthAddress: gethcrypto.PubkeyToAddress(key.PublicKey).Hex(),
		PrivateKey: "0x" + hex.EncodeToString(gethcrypto.FromECDSA(key)),
	}
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestDepositAndBridge_Live -v
// Chains/tokens come from env: HINKAL_BRIDGE_IN_CHAIN / _IN_TOKEN (default HINKAL_CHAIN_ID / HINKAL_TOKEN)
// and HINKAL_BRIDGE_OUT_CHAIN / _OUT_TOKEN (both required), plus HINKAL_BRIDGE_AMOUNT.
func TestDepositAndBridge_Live(t *testing.T) {
	requireLive(t)
	defaultChainID, defaultToken := evmTestChainAndToken(t)
	sourceChainID := envChainID(t, "HINKAL_BRIDGE_IN_CHAIN", defaultChainID)
	sourceTokenAddress := envOr("HINKAL_BRIDGE_IN_TOKEN", defaultToken)
	if sourceChainID != defaultChainID && os.Getenv("HINKAL_BRIDGE_IN_TOKEN") == "" {
		t.Fatalf("HINKAL_BRIDGE_IN_CHAIN=%d differs from the default chain; set HINKAL_BRIDGE_IN_TOKEN", sourceChainID)
	}
	destinationChainID := envChainID(t, "HINKAL_BRIDGE_OUT_CHAIN", 0)
	destinationTokenAddress := os.Getenv("HINKAL_BRIDGE_OUT_TOKEN")
	if destinationChainID == 0 || destinationTokenAddress == "" {
		t.Skip("set HINKAL_BRIDGE_OUT_CHAIN and HINKAL_BRIDGE_OUT_TOKEN to run the bridge test")
	}
	amount := envOr("HINKAL_BRIDGE_AMOUNT", "0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, sourceChainID)
	tokens, err := web3.ResolveERC20Tokens(ctx, sourceChainID, []string{sourceTokenAddress})
	if err != nil {
		t.Fatalf("resolve source token: %v", err)
	}
	sourceToken := tokens[0]
	destinationTokens, err := web3.ResolveERC20Tokens(ctx, destinationChainID, []string{destinationTokenAddress})
	if err != nil {
		t.Fatalf("resolve destination token: %v", err)
	}
	destinationToken := destinationTokens[0]
	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		t.Fatalf("bridge amount: %v", err)
	}

	tempAccount := temporarySubAccount(t)
	quote, err := web3.GetLifiPrice(ctx, sourceToken, destinationToken, amount, 0.005, tempAccount.EthAddress, ethAddress)
	if err != nil {
		t.Fatalf("lifi quote: %v", err)
	}
	t.Logf("lifi quote: in=%s expected=%s nativeFee=%s temp=%s", bridgeAmount, quote.ExpectedAmount, quote.NativeFee, tempAccount.EthAddress)

	result, err := h.DepositAndBridge(ctx, sourceChainID, sourceTokenAddress, []types.BridgeRecipient{
		{
			RecipientAddress:    ethAddress,
			BridgeAmount:        bridgeAmount,
			Quote:               quote,
			TemporarySubAccount: tempAccount,
		},
	}, nil, nil, true)
	if err != nil {
		t.Fatalf("deposit and bridge: %v", err)
	}
	if result.DepositTxHash == "" {
		t.Fatalf("deposit tx hash is empty")
	}
	if result.ScheduleID == "" {
		t.Fatalf("schedule id is empty")
	}
	t.Logf("deposit and bridge: deposit=%s schedule=%s amount=%s", result.DepositTxHash, result.ScheduleID, bridgeAmount)
}
