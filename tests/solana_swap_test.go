package tests

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
)

func solanaSwapConfig() (outMint, amount string) {
	outMint = os.Getenv("HINKAL_SOLANA_SWAP_OUT_TOKEN")
	if outMint == "" {
		outMint = constants.SolanaNativeAddress // default: swap USDC -> SOL
	}
	amount = os.Getenv("HINKAL_SOLANA_SWAP_AMOUNT")
	if amount == "" {
		amount = "1" // Solana relayer fees are charged from the output token.
	}
	return outMint, amount
}

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 go test ./tests/ -run TestSolanaSwap_Live -v
func TestSolanaSwap_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	chainID := constants.CurrentSolanaChainID
	outMint, amount := solanaSwapConfig()

	h := newLiveSolanaHinkal(t, ctx)
	tokens, err := web3.ResolveERC20Tokens(ctx, chainID, []string{solanaMainnetUSDC, outMint})
	if err != nil {
		t.Fatalf("resolve tokens: %v", err)
	}
	inToken, outToken := tokens[0], tokens[1]
	t.Logf("solana swap config: chain=%d amount=%s in=%s(%d) out=%s(%d)", chainID, amount, inToken.Symbol, inToken.Decimals, outToken.Symbol, outToken.Decimals)

	inAmountWei, err := web3.GetAmountInWei(inToken, amount)
	if err != nil {
		t.Fatalf("in amount: %v", err)
	}

	quote, err := web3.GetOKXPrice(ctx, chainID, inToken, outToken, amount, 0.7)
	if err != nil {
		t.Fatalf("okx quote: %v", err)
	}
	t.Logf("okx quote: in=%s out=%s", inAmountWei, quote.OutSwapAmount)

	sig, err := h.DepositSolana(ctx, chainID, solanaMainnetUSDC, inAmountWei, false)
	if err != nil {
		t.Fatalf("solana deposit before swap: %v", err)
	}
	if _, err := h.WaitForTransaction(ctx, chainID, sig, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	outBefore := solanaPrivateBalance(t, ctx, h, outMint)
	deltaAmounts := []*big.Int{new(big.Int).Neg(inAmountWei), quote.OutSwapAmount}
	swapSig, err := h.Swap(ctx, chainID, []string{solanaMainnetUSDC, outMint}, deltaAmounts, "", quote.OKXData, "", nil)
	if err != nil {
		t.Fatalf("solana swap: %v", err)
	}
	t.Logf("solana swap sig: %s", swapSig)
	if _, err := h.WaitForTransaction(ctx, chainID, swapSig, 1); err != nil {
		t.Fatalf("wait for swap tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	outAfter := solanaPrivateBalance(t, ctx, h, outMint)
	delta := new(big.Int).Sub(outAfter, outBefore)
	t.Logf("solana private %s after swap: before=%s after=%s delta=%s", outMint, outBefore, outAfter, delta)
	if delta.Sign() <= 0 {
		t.Fatalf("expected output token private balance to increase, got delta=%s", delta)
	}
}
