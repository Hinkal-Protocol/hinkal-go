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

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestTransfer_Live -v
func TestTransfer_Live(t *testing.T) {
	requireLive(t)
	chainID, token := evmTestChainAndToken(t)
	transferAmount := big.NewInt(10_000) // 0.05 USDC (6 decimals)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	privateBeforeDeposit := privateBalanceForToken(t, ctx, h, chainID, ethAddress, token)

	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		token,
		[]string{token},
		types.ExternalActionTransact,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}

	if feeStructure.VariableRate == nil || feeStructure.VariableRate.Sign() == 0 {
		feeStructure.VariableRate = constants.HinkalPrivateSendVariableRate()
	}
	totalRelayFee := fees.CalculateTotalFee(transferAmount, feeStructure)
	depositAmount := new(big.Int).Add(transferAmount, totalRelayFee)

	_, depositTxHash, err := h.Deposit(ctx, chainID, []string{token}, []*big.Int{depositAmount}, true, false)
	if err != nil {
		t.Fatalf("deposit before transfer: %v", err)
	}
	t.Logf("deposit tx: %s (amount=%s)", depositTxHash, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, token, privateBeforeDeposit, depositAmount)

	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	privateBefore := privateBalanceForToken(t, ctx, h, chainID, ethAddress, token)
	transferChange := new(big.Int).Neg(transferAmount)
	transferTxHash, err := h.Transfer(ctx, chainID, []string{token}, []*big.Int{transferChange}, recipientInfo, token, &feeStructure)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	t.Logf("transfer tx: %s (amount=%s fee=%s)", transferTxHash, transferAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, transferTxHash, 1); err != nil {
		t.Fatalf("wait for transfer tx: %v", err)
	}
	expected := new(big.Int).Neg(totalRelayFee)
	privateAfter := waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, token, privateBefore, expected)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	t.Logf("private USDC transfer: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
