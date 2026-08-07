package tests

import (
	"context"
	"math/big"
	"testing"
	"time"
)

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestDepositAndWithdraw_Live -v
func TestDepositAndWithdraw_Live(t *testing.T) {
	requireLive(t)
	chainID, token := evmTestChainAndToken(t)
	recipientAmount := big.NewInt(10_000) // 0.01 USDC (6 decimals)

	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	result, err := h.DepositAndWithdraw(
		ctx,
		chainID,
		token,
		[]*big.Int{recipientAmount},
		[]string{ethAddress},
		nil,
		nil,
		true,
	)
	if err != nil {
		t.Fatalf("deposit and withdraw: %v", err)
	}
	if result.DepositTxHash == "" {
		t.Fatalf("deposit tx hash is empty")
	}
	if result.ScheduleID == "" {
		t.Fatalf("schedule id is empty")
	}
	t.Logf("deposit and withdraw: deposit=%s schedule=%s amount=%s", result.DepositTxHash, result.ScheduleID, recipientAmount)
}
