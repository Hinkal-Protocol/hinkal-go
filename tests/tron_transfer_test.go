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

// HINKAL_LIVE=1 HINKAL_TRON_PRIVATE_KEY=... go test ./tests/ -run TestTronTransfer_Live -v
func TestTronTransfer_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.TronNile
	transferAmount := big.NewInt(50_000) // 0.05 USDT

	h, tronAddress := newLiveTronHinkal(t, ctx, chainID)

	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tronNileUSDT,
		[]string{tronNileUSDT},
		types.ExternalActionTransact,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	totalRelayFee := fees.CalculateTotalFee(transferAmount, feeStructure)
	depositAmount := new(big.Int).Add(transferAmount, totalRelayFee)

	_, depositTxid, err := h.Deposit(ctx, chainID, []string{tronNileUSDT}, []*big.Int{depositAmount}, true, false)
	if err != nil {
		t.Fatalf("tron deposit before transfer: %v", err)
	}
	t.Logf("tron deposit txid: %s (amount=%s)", depositTxid, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxid, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	privateBefore := tronPrivateBalance(t, ctx, h, chainID, tronAddress, tronNileUSDT)
	transferChange := new(big.Int).Neg(transferAmount)
	transferTxid, err := h.Transfer(ctx, chainID, []string{tronNileUSDT}, []*big.Int{transferChange}, recipientInfo, tronNileUSDT, &feeStructure)
	if err != nil {
		t.Fatalf("tron transfer: %v", err)
	}
	t.Logf("tron transfer txid: %s (amount=%s fee=%s)", transferTxid, transferAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, transferTxid, 1); err != nil {
		t.Fatalf("wait for transfer tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateAfter := tronPrivateBalance(t, ctx, h, chainID, tronAddress, tronNileUSDT)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	expected := new(big.Int).Neg(totalRelayFee)
	t.Logf("tron private USDT transfer: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
