package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
)

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 go test ./tests/ -run TestSolanaTransfer_Live -v
func TestSolanaTransfer_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	chainID := constants.CurrentSolanaChainID
	transferAmount := big.NewInt(10_000) // 0.01 USDC (6 decimals)

	h := newLiveSolanaHinkal(t, ctx)

	amountChanges := []*big.Int{new(big.Int).Neg(transferAmount)}
	preFeeStructure := solanaFreshFeeStructure(t, ctx, h, chainID, amountChanges)
	preTotalFee := fees.CalculateTotalFee(transferAmount, preFeeStructure)
	depositAmount := new(big.Int).Add(transferAmount, new(big.Int).Mul(preTotalFee, big.NewInt(2)))

	sig, err := h.DepositSolana(ctx, chainID, solanaMainnetUSDC, depositAmount, false)
	if err != nil {
		t.Fatalf("solana deposit before transfer: %v", err)
	}
	t.Logf("solana deposit sig: %s (amount=%s)", sig, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, sig, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateBefore := solanaPrivateBalance(t, ctx, h, solanaMainnetUSDC)
	feeStructure := solanaFreshFeeStructure(t, ctx, h, chainID, amountChanges)
	totalRelayFee := fees.CalculateTotalFee(transferAmount, feeStructure)

	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	transferChange := new(big.Int).Neg(transferAmount)
	transferSig, err := h.Transfer(ctx, chainID, []string{solanaMainnetUSDC}, []*big.Int{transferChange}, recipientInfo, solanaMainnetUSDC, &feeStructure)
	if err != nil {
		t.Fatalf("solana transfer: %v", err)
	}
	t.Logf("solana transfer sig: %s (amount=%s fee=%s)", transferSig, transferAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, transferSig, 1); err != nil {
		t.Fatalf("wait for transfer tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateAfter := solanaPrivateBalance(t, ctx, h, solanaMainnetUSDC)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	expected := new(big.Int).Neg(totalRelayFee)
	t.Logf("solana private USDC transfer: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
