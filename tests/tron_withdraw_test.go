package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// HINKAL_LIVE=1 HINKAL_TRON_PRIVATE_KEY=... go test ./tests/ -run TestTronWithdraw_Live -v
func TestTronWithdraw_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.TronNile
	withdrawAmount := big.NewInt(10_000) // 0.01 USDT

	h, tronAddress := newLiveTronHinkal(t, ctx, chainID)

	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, tronNileUSDT, []string{tronNileUSDT}, types.ExternalActionTransact, nil, nil, nil)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	totalRelayFee := fees.CalculateTotalFee(withdrawAmount, feeStructure)
	depositAmount := new(big.Int).Add(withdrawAmount, totalRelayFee)

	_, depositTxid, err := h.Deposit(ctx, chainID, []string{tronNileUSDT}, []*big.Int{depositAmount}, true, false)
	if err != nil {
		t.Fatalf("tron deposit before withdraw: %v", err)
	}
	t.Logf("tron deposit txid: %s (amount=%s)", depositTxid, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxid, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateBefore := tronPrivateBalance(t, ctx, h, chainID, tronAddress, tronNileUSDT)
	withdrawChange := new(big.Int).Neg(withdrawAmount)
	_, withdrawTxid, err := h.Withdraw(ctx, chainID, []string{tronNileUSDT}, []*big.Int{withdrawChange}, tronAddress, false, tronNileUSDT, &feeStructure)
	if err != nil {
		t.Fatalf("tron withdraw: %v", err)
	}
	t.Logf("tron withdraw txid: %s (amount=%s fee=%s)", withdrawTxid, withdrawAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, withdrawTxid, 1); err != nil {
		t.Fatalf("wait for withdraw tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateAfter := tronPrivateBalance(t, ctx, h, chainID, tronAddress, tronNileUSDT)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	expected := new(big.Int).Neg(new(big.Int).Add(withdrawAmount, totalRelayFee))
	t.Logf("tron private USDT withdraw: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
