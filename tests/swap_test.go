package tests

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const evmSwapSlippagePercent = 0.7

type evmSwapQuote struct {
	outAmount        *big.Int
	swapData         string
	externalActionID types.ExternalActionID
}

type evmSwapQuoteProvider struct {
	name  string
	quote func(context.Context, *ethclient.Client, int, string, types.ERC20Token, types.ERC20Token) (evmSwapQuote, error)
}

func swapEnv(t *testing.T) (chainID int, inToken, outToken, amount string) {
	t.Helper()
	defaultChainID, defaultInToken := evmTestChainAndToken(t)
	chainID = envChainID(t, "HINKAL_SWAP_CHAIN", defaultChainID)

	inToken = os.Getenv("HINKAL_SWAP_IN_TOKEN")
	if inToken == "" {
		if chainID != defaultChainID {
			t.Fatalf("HINKAL_SWAP_CHAIN=%d differs from the default chain; set HINKAL_SWAP_IN_TOKEN to a token on that chain", chainID)
		}
		inToken = defaultInToken
	}

	outToken = os.Getenv("HINKAL_SWAP_OUT_TOKEN")
	if outToken == "" {
		t.Skip("set HINKAL_SWAP_OUT_TOKEN to the token address to swap into")
	}

	amount = envOr("HINKAL_SWAP_AMOUNT", "0.1")
	return chainID, inToken, outToken, amount
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func evmSwapQuoteProviders() []evmSwapQuoteProvider {
	return []evmSwapQuoteProvider{
		{
			name: "lifi",
			quote: func(ctx context.Context, _ *ethclient.Client, chainID int, amount string, inToken, outToken types.ERC20Token) (evmSwapQuote, error) {
				externalAddress, err := pretransaction.GetExternalSwapAddress(chainID, types.ExternalActionLifi)
				if err != nil {
					return evmSwapQuote{}, err
				}
				quote, err := web3.GetLifiPrice(ctx, inToken, outToken, amount, evmSwapSlippagePercent*0.01, externalAddress, externalAddress)
				if err != nil {
					return evmSwapQuote{}, err
				}
				return evmSwapQuote{
					outAmount:        quote.ExpectedAmount,
					swapData:         quote.Calldata,
					externalActionID: types.ExternalActionLifi,
				}, nil
			},
		},
	}
}

func runEVMSwapProviderLive(
	t *testing.T,
	ctx context.Context,
	h *hinkal.Hinkal,
	client *ethclient.Client,
	provider evmSwapQuoteProvider,
	chainID int,
	ethAddress string,
	inTokenAddr string,
	outTokenAddr string,
	amount string,
) {
	t.Helper()

	tokens, err := web3.ResolveERC20Tokens(ctx, chainID, []string{inTokenAddr, outTokenAddr})
	if err != nil {
		t.Fatalf("resolve tokens: %v", err)
	}
	inToken, outToken := tokens[0], tokens[1]
	t.Logf("swap config: provider=%s chain=%d amount=%s in=%s(%d) out=%s(%d)", provider.name, chainID, amount, inToken.Symbol, inToken.Decimals, outToken.Symbol, outToken.Decimals)

	inAmountWei, err := web3.GetAmountInWei(inToken, amount)
	if err != nil {
		t.Fatalf("in amount: %v", err)
	}

	quote, err := provider.quote(ctx, client, chainID, amount, inToken, outToken)
	if err != nil {
		t.Fatalf("%s quote: %v", provider.name, err)
	}
	t.Logf("%s quote: in=%s out=%s", provider.name, inAmountWei, quote.outAmount)
	inBefore := privateBalanceForToken(t, ctx, h, chainID, ethAddress, inTokenAddr)

	_, depositTxHash, err := h.Deposit(ctx, chainID, []string{inTokenAddr}, []*big.Int{inAmountWei}, true, false)
	if err != nil {
		t.Fatalf("deposit before swap: %v", err)
	}
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, inTokenAddr, inBefore, inAmountWei)

	outBefore := privateBalanceForToken(t, ctx, h, chainID, ethAddress, outTokenAddr)
	deltaAmounts := []*big.Int{new(big.Int).Neg(inAmountWei), quote.outAmount}
	swapTxHash, err := h.Swap(ctx, chainID, []string{inTokenAddr, outTokenAddr}, deltaAmounts, quote.externalActionID, quote.swapData, inTokenAddr, nil)
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	t.Logf("swap tx: %s", swapTxHash)
	if _, err := h.WaitForTransaction(ctx, chainID, swapTxHash, 1); err != nil {
		t.Fatalf("wait for swap tx: %v", err)
	}
	outAfter := waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, outTokenAddr, outBefore, quote.outAmount)
	delta := new(big.Int).Sub(outAfter, outBefore)
	t.Logf("private %s after swap: before=%s after=%s delta=%s", outTokenAddr, outBefore, outAfter, delta)
	if delta.Sign() <= 0 {
		t.Fatalf("expected output token private balance to increase, got delta=%s", delta)
	}
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/ -run TestSwap_Live -v
// Chain/tokens come from env: HINKAL_SWAP_CHAIN / _IN_TOKEN / _OUT_TOKEN / _AMOUNT
// (chain and in-token default to HINKAL_CHAIN_ID / HINKAL_TOKEN; out-token is required)
func TestSwap_Live(t *testing.T) {
	requireLive(t)
	chainID, inTokenAddr, outTokenAddr, amount := swapEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	client, err := h.GetFetchClient(chainID)
	if err != nil {
		t.Fatalf("fetch client: %v", err)
	}

	for _, provider := range evmSwapQuoteProviders() {
		provider := provider
		t.Run(provider.name, func(t *testing.T) {
			runEVMSwapProviderLive(t, ctx, h, client, provider, chainID, ethAddress, inTokenAddr, outTokenAddr, amount)
		})
	}
}
