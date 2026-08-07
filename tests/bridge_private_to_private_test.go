package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func amountInWeiForDeposit(t *testing.T, ctx context.Context, chainID int, tokenAddress, amount string) (*big.Int, error) {
	t.Helper()
	tokens, err := web3.ResolveERC20Tokens(ctx, chainID, []string{tokenAddress})
	if err != nil {
		return nil, err
	}
	return web3.GetAmountInWei(tokens[0], amount)
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestBridgePrivateToPrivate_Live -v
// Defaults to bridging 0.2 USDC from Base to Polygon; override with:
// HINKAL_PRIVATE_BRIDGE_IN_CHAIN / _IN_TOKEN, HINKAL_PRIVATE_BRIDGE_OUT_CHAIN / _OUT_TOKEN, HINKAL_PRIVATE_BRIDGE_AMOUNT.
func TestBridgePrivateToPrivate_Live(t *testing.T) {
	requireLive(t)

	sourceChainID := envChainID(t, "HINKAL_PRIVATE_BRIDGE_IN_CHAIN", 8453) // Base
	destChainID := envChainID(t, "HINKAL_PRIVATE_BRIDGE_OUT_CHAIN", 137)   // Polygon
	sourceTokenAddress := envOr("HINKAL_PRIVATE_BRIDGE_IN_TOKEN", "")
	destTokenAddress := envOr("HINKAL_PRIVATE_BRIDGE_OUT_TOKEN", "")
	if sourceTokenAddress == "" || destTokenAddress == "" {
		t.Skip("set HINKAL_PRIVATE_BRIDGE_IN_TOKEN and HINKAL_PRIVATE_BRIDGE_OUT_TOKEN (USDC addresses) to run this test")
	}
	amount := envOr("HINKAL_PRIVATE_BRIDGE_AMOUNT", "0.2")

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, sourceChainID)

	shieldAmount, err := amountInWeiForDeposit(t, ctx, sourceChainID, sourceTokenAddress, amount)
	if err != nil {
		t.Fatalf("compute deposit amount: %v", err)
	}
	privateBeforeDeposit := privateBalanceForToken(t, ctx, h, sourceChainID, ethAddress, sourceTokenAddress)
	_, depositTxHash, err := h.Deposit(ctx, sourceChainID, []string{sourceTokenAddress}, []*big.Int{shieldAmount}, true, false)
	if err != nil {
		t.Fatalf("deposit (shield) before bridge: %v", err)
	}
	if _, err := h.WaitForTransaction(ctx, sourceChainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	waitForPrivateBalanceDelta(t, ctx, h, sourceChainID, ethAddress, sourceTokenAddress, privateBeforeDeposit, shieldAmount)

	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		t.Fatalf("get recipient info: %v", err)
	}

	result, err := h.BridgePrivateToPrivate(
		ctx,
		sourceChainID,
		sourceTokenAddress,
		destChainID,
		destTokenAddress,
		amount,
		bridging.PrivateBridgeRecipient{RecipientInfo: recipientInfo},
		0.01,
		"",
	)
	if err != nil {
		t.Fatalf("bridge private to private: %v", err)
	}
	if result.SourceTxHash == "" {
		t.Fatalf("source tx hash is empty")
	}
	if result.DestTxHash == "" {
		t.Fatalf("dest tx hash is empty")
	}
	t.Logf("bridge private to private: source=%s dest=%s", result.SourceTxHash, result.DestTxHash)
}
