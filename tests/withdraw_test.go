package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestWithdraw_Live -v
func TestWithdraw_Live(t *testing.T) {
	requireLive(t)
	chainID, token := evmTestChainAndToken(t)
	withdrawAmount := big.NewInt(10_000) // 0.01 USDC (6 decimals)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	privateBeforeDeposit := privateBalanceForToken(t, ctx, h, chainID, ethAddress, token)

	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, token, []string{token}, types.ExternalActionTransact, nil, nil, nil)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	totalRelayFee := fees.CalculateTotalFee(withdrawAmount, feeStructure)
	depositAmount := new(big.Int).Add(withdrawAmount, totalRelayFee)

	_, depositTxHash, err := h.Deposit(ctx, chainID, []string{token}, []*big.Int{depositAmount}, true, false)
	if err != nil {
		t.Fatalf("deposit before withdraw: %v", err)
	}
	t.Logf("deposit tx: %s (amount=%s)", depositTxHash, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	privateBefore := waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, token, privateBeforeDeposit, depositAmount)
	withdrawChange := new(big.Int).Neg(withdrawAmount)
	_, withdrawTxHash, err := h.Withdraw(ctx, chainID, []string{token}, []*big.Int{withdrawChange}, ethAddress, false, token, &feeStructure)
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	t.Logf("withdraw tx: %s (amount=%s fee=%s)", withdrawTxHash, withdrawAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, withdrawTxHash, 1); err != nil {
		t.Fatalf("wait for withdraw tx: %v", err)
	}
	expected := new(big.Int).Neg(new(big.Int).Add(withdrawAmount, totalRelayFee))
	privateAfter := waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, token, privateBefore, expected)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	t.Logf("private USDC withdraw: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
