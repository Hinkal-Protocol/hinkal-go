package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

// HINKAL_LIVE=1 HINKAL_TRON_PRIVATE_KEY=... go test ./tests/ -run TestTronDepositAndWithdraw_Live -v
func TestTronDepositAndWithdraw_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.TronNile
	recipientAmount := big.NewInt(300_000) // 0.3 USDT

	h, tronAddress := newLiveTronHinkal(t, ctx, chainID)
	result, err := h.DepositAndWithdraw(
		ctx,
		chainID,
		tronNileUSDT,
		[]*big.Int{recipientAmount},
		[]string{tronAddress},
		nil,
		nil,
		true,
	)
	if err != nil {
		t.Fatalf("tron deposit and withdraw: %v", err)
	}
	if result.DepositTxHash == "" {
		t.Fatalf("deposit txid is empty")
	}
	if result.ScheduleID == "" {
		t.Fatalf("schedule id is empty")
	}
	t.Logf("tron deposit and withdraw: deposit=%s schedule=%s amount=%s", result.DepositTxHash, result.ScheduleID, recipientAmount)
}
