package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

func TestTronDeposit_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.TronNile
	h, tronAddress := newLiveTronHinkal(t, ctx, chainID)
	token, amount := tronDepositToken(t)
	before := tronPrivateBalance(t, ctx, h, chainID, tronAddress, token)

	_, txid, err := h.Deposit(ctx, chainID, []string{token}, []*big.Int{amount}, true, false)
	if err != nil {
		t.Fatalf("tron deposit: %v", err)
	}
	t.Logf("tron deposit txid: %s", txid)
	if _, err := h.WaitForTransaction(ctx, chainID, txid, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	after := tronPrivateBalance(t, ctx, h, chainID, tronAddress, token)
	delta := new(big.Int).Sub(after, before)
	t.Logf("tron private %s: before=%s after=%s delta=%s", token, before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}

func TestTronDepositForOther_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.TronNile
	h, tronAddress := newLiveTronHinkal(t, ctx, chainID)
	token, amount := tronDepositToken(t)
	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	before := tronPrivateBalance(t, ctx, h, chainID, tronAddress, token)

	_, txid, err := h.DepositForOther(ctx, chainID, []string{token}, []*big.Int{amount}, recipientInfo, true, false)
	if err != nil {
		t.Fatalf("tron deposit for other: %v", err)
	}
	t.Logf("tron deposit-for-other txid: %s", txid)
	if _, err := h.WaitForTransaction(ctx, chainID, txid, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	after := tronPrivateBalance(t, ctx, h, chainID, tronAddress, token)
	delta := new(big.Int).Sub(after, before)
	t.Logf("tron private %s: before=%s after=%s delta=%s", token, before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}
