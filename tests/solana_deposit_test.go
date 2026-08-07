package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 \
// [HINKAL_SOLANA_TOKEN=mint HINKAL_SOLANA_AMOUNT=units] go test ./tests/ -run TestSolanaDeposit_Live -v
func TestSolanaDeposit_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	h := newLiveSolanaHinkal(t, ctx)
	token, amount := solanaDepositToken(t)
	before := solanaPrivateBalance(t, ctx, h, token)

	sig, err := h.DepositSolana(ctx, constants.CurrentSolanaChainID, token, amount, false)
	if err != nil {
		t.Fatalf("solana deposit: %v", err)
	}
	t.Logf("solana deposit sig: %s", sig)
	if _, err := h.WaitForTransaction(ctx, constants.CurrentSolanaChainID, sig, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(15 * time.Second) // let the snapshot server catch up

	after := solanaPrivateBalance(t, ctx, h, token)
	delta := new(big.Int).Sub(after, before)
	t.Logf("solana private %s: before=%s after=%s delta=%s", token, before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 go test ./tests/ -run TestSolanaDepositForOther_Live -v
func TestSolanaDepositForOther_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	h := newLiveSolanaHinkal(t, ctx)
	token, amount := solanaDepositToken(t)
	recipientInfo, err := h.GetRecipientInfo() // deposit to our own shielded account
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	before := solanaPrivateBalance(t, ctx, h, token)

	sig, err := h.DepositSolanaForOther(ctx, constants.CurrentSolanaChainID, token, amount, recipientInfo, false)
	if err != nil {
		t.Fatalf("solana deposit for other: %v", err)
	}
	t.Logf("solana deposit-for-other sig: %s", sig)
	if _, err := h.WaitForTransaction(ctx, constants.CurrentSolanaChainID, sig, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	after := solanaPrivateBalance(t, ctx, h, token)
	delta := new(big.Int).Sub(after, before)
	t.Logf("solana private %s: before=%s after=%s delta=%s", token, before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}

func TestSolanaProoflessDeposit_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	h := newLiveSolanaHinkal(t, ctx)
	token, amount := solanaDepositToken(t)
	before := solanaPrivateBalance(t, ctx, h, token)

	_, sig, err := h.ProoflessDeposit(ctx, constants.CurrentSolanaChainID, []string{token}, []*big.Int{amount}, nil, nil, false, types.ZeroProoflessFeeStructure(), "", false)
	if err != nil {
		t.Fatalf("solana proofless deposit: %v", err)
	}
	t.Logf("solana proofless deposit sig: %s", sig)
	if _, err := h.WaitForTransaction(ctx, constants.CurrentSolanaChainID, sig, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	after := solanaPrivateBalance(t, ctx, h, token)
	delta := new(big.Int).Sub(after, before)
	t.Logf("solana private %s: before=%s after=%s delta=%s", token, before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}
