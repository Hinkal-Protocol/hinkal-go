package tests

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/transactions"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func stuckBalanceForToken(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, ethAddress, tokenAddress string) *big.Int {
	t.Helper()
	balances, err := h.GetStuckShieldedBalances(ctx, chainID, nil, ethAddress)
	if err != nil {
		t.Fatalf("get stuck shielded balances: %v", err)
	}
	for _, b := range balances {
		if strings.EqualFold(b.Token.Erc20TokenAddress, tokenAddress) {
			return b.Balance
		}
	}
	return new(big.Int)
}

func waitForStuckBalanceToClear(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, ethAddress, tokenAddress string) *big.Int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		balance := stuckBalanceForToken(t, ctx, h, chainID, ethAddress, tokenAddress)
		if balance.Sign() == 0 {
			return balance
		}
		if time.Now().After(deadline) {
			t.Fatalf("stuck balance did not clear within 2m: last balance=%s", balance)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context done while waiting for stuck balance to clear: %v", ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
}

func waitForStuckBalanceForToken(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, ethAddress, tokenAddress string) *big.Int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		balance := stuckBalanceForToken(t, ctx, h, chainID, ethAddress, tokenAddress)
		if balance.Sign() > 0 {
			return balance
		}
		if time.Now().After(deadline) {
			return balance
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context done while waiting for stuck balance: %v", ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
}

func stuckDepositAmount(t *testing.T) *big.Int {
	t.Helper()
	amount := big.NewInt(300_000) // 0.3 USDC (6 decimals)
	if v := os.Getenv("HINKAL_STUCK_DEPOSIT_AMOUNT"); v != "" {
		parsed, ok := new(big.Int).SetString(v, 10)
		if !ok {
			t.Fatalf("bad HINKAL_STUCK_DEPOSIT_AMOUNT: %q", v)
		}
		amount = parsed
	}
	return amount
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestWithdrawStuckUtxos_Live -v
func TestWithdrawStuckUtxos_Live(t *testing.T) {
	requireLive(t)

	chainID, tokenAddress := evmTestChainAndToken(t)
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	resolvedTokens, err := web3.ResolveERC20Tokens(ctx, chainID, []string{tokenAddress})
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	token := resolvedTokens[0]
	depositAmount := stuckDepositAmount(t)

	ethAddressHex, err := utils.AddressToHexFormat(ethAddress)
	if err != nil {
		t.Fatalf("address to hex: %v", err)
	}
	hashedEthereumAddress := utils.HashEthereumAddress(ethAddressHex)
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tokenAddress,
		[]string{tokenAddress},
		types.ExternalActionTransact,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}

	userDepositedUtxos, statusID, depositTxHash, err := transactions.HinkalDepositOnChainUtxos(
		ctx,
		h,
		chainID,
		token,
		[]*big.Int{depositAmount},
		[]string{ethAddress},
		feeStructure,
		hashedEthereumAddress,
		true,
	)
	if err != nil {
		t.Fatalf("deposit on-chain UTXOs: %v", err)
	}
	if len(userDepositedUtxos) == 0 {
		t.Fatalf("deposited UTXOs are empty")
	}
	if depositTxHash == "" {
		t.Fatalf("deposit tx hash is empty")
	}
	if statusID == "" {
		t.Fatalf("status id is empty")
	}
	t.Logf("deposit on-chain UTXOs: deposit=%s status=%s amount=%s", depositTxHash, statusID, depositAmount)

	stuckBefore := waitForStuckBalanceForToken(t, ctx, h, chainID, ethAddress, tokenAddress)
	if stuckBefore.Sign() <= 0 {
		t.Fatalf("stuck balance after deposit = %s, want positive (deposit=%s status=%s)", stuckBefore, depositTxHash, statusID)
	}

	txHashes, err := h.WithdrawStuckUtxos(ctx, chainID, tokenAddress, ethAddress)
	if err != nil {
		t.Fatalf("withdraw stuck UTXOs: stuckBefore=%s deposit=%s status=%s err=%v", stuckBefore, depositTxHash, statusID, err)
	}
	if len(txHashes) == 0 {
		t.Fatalf("withdraw stuck UTXOs returned no transactions")
	}
	for _, txHash := range txHashes {
		if !strings.HasPrefix(txHash, "0x") || len(txHash) != 66 {
			t.Fatalf("unexpected EVM tx hash %q", txHash)
		}
		if _, err := h.WaitForTransaction(ctx, chainID, txHash, 1); err != nil {
			t.Fatalf("wait for stuck-withdraw tx %s: %v", txHash, err)
		}
	}

	stuckAfter := waitForStuckBalanceToClear(t, ctx, h, chainID, ethAddress, tokenAddress)
	t.Logf("withdraw stuck UTXOs: deposit=%s status=%s stuckBefore=%s stuckAfter=%s txs=%v", depositTxHash, statusID, stuckBefore, stuckAfter, txHashes)
	if stuckAfter.Sign() != 0 {
		t.Fatalf("stuck balance after withdraw = %s, want 0", stuckAfter)
	}
}
