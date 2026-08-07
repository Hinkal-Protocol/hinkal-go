package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 go test ./tests/ -run TestSolanaDepositAndWithdraw_Live -v
func TestSolanaDepositAndWithdraw_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	chainID := constants.CurrentSolanaChainID
	recipientAmount := big.NewInt(10_000) // 0.01 USDC (6 decimals)

	h := newLiveSolanaHinkal(t, ctx)
	recipient, err := h.GetSolanaPublicKey(ctx)
	if err != nil {
		t.Fatalf("solana recipient: %v", err)
	}
	result, err := h.DepositAndWithdraw(
		ctx,
		chainID,
		solanaMainnetUSDC,
		[]*big.Int{recipientAmount},
		[]string{recipient.String()},
		nil,
		nil,
		true,
	)
	if err != nil {
		t.Fatalf("solana deposit and withdraw: %v", err)
	}
	if result.DepositTxHash == "" {
		t.Fatalf("deposit signature is empty")
	}
	if result.ScheduleID == "" {
		t.Fatalf("schedule id is empty")
	}
	t.Logf("solana deposit and withdraw: deposit=%s schedule=%s amount=%s", result.DepositTxHash, result.ScheduleID, recipientAmount)
}
