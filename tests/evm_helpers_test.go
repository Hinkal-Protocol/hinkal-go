package tests

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/signers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const arcTestnetUSDC = "0x3600000000000000000000000000000000000000"

func livePrivateKey(t *testing.T) string {
	t.Helper()
	pk := os.Getenv("HINKAL_PRIVATE_KEY")
	if pk == "" {
		t.Skip("set HINKAL_PRIVATE_KEY to a funded wallet key to run the deposit test")
	}
	return pk
}

func evmTestChainAndToken(t *testing.T) (int, string) {
	t.Helper()
	chainID := constants.ChainIDs.ArcTestnet
	if v := os.Getenv("HINKAL_CHAIN_ID"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("bad HINKAL_CHAIN_ID: %v", err)
		}
		chainID = parsed
	}
	token := os.Getenv("HINKAL_TOKEN")
	if token == "" {
		if chainID != constants.ChainIDs.ArcTestnet {
			t.Fatalf("HINKAL_CHAIN_ID=%d overrides the default chain; set HINKAL_TOKEN to a token address on that chain", chainID)
		}
		token = arcTestnetUSDC
	}
	return chainID, token
}

// newLiveEVMHinkal wires a private-key signer + EVM provider adapter into a fresh Hinkal
// and derives the user keys from the protocol login signature.
// envChainID reads an optional chain-id override, falling back to the supplied default.
func envChainID(t *testing.T, key string, fallback int) int {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("bad %s: %q", key, v)
	}
	return parsed
}

func newLiveEVMHinkal(t *testing.T, ctx context.Context, chainID int) (*hinkal.Hinkal, string) {
	t.Helper()
	signer, err := signers.NewPrivateKeyEVMSigner(livePrivateKey(t))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	adapter, err := providers.NewEthersProviderAdapter()
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	adapter.InitSigner(signer)
	cid := chainID
	if err := adapter.Init(&cid); err != nil {
		t.Fatalf("adapter init: %v", err)
	}

	h := hinkal.NewHinkal(nil)
	if err := h.InitProviderAdapter(ctx, adapter); err != nil {
		t.Fatalf("init provider adapter: %v", err)
	}

	switch {
	case os.Getenv("HINKAL_SIGNATURE") != "":
		h.InitUserKeysWithSignature(os.Getenv("HINKAL_SIGNATURE"))
	case os.Getenv("HINKAL_SEED_PHRASE") != "":
		if err := h.InitUserKeysFromSeedPhrases(strings.Fields(os.Getenv("HINKAL_SEED_PHRASE"))); err != nil {
			t.Fatalf("init user keys from seed phrase: %v", err)
		}
	default:
		if err := h.InitUserKeys(ctx, types.LoginMessageModeProtocol); err != nil {
			t.Fatalf("init user keys: %v", err)
		}
	}
	ethAddress, err := h.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		t.Fatalf("eth address: %v", err)
	}
	return h, ethAddress
}

func privateBalanceForToken(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, ethAddress, tokenAddress string) *big.Int {
	t.Helper()
	balance, err := fetchPrivateBalanceForToken(ctx, h, chainID, ethAddress, tokenAddress)
	if err != nil {
		t.Fatalf("get total balance: %v", err)
	}
	return balance
}

func fetchPrivateBalanceForToken(ctx context.Context, h *hinkal.Hinkal, chainID int, ethAddress, tokenAddress string) (*big.Int, error) {
	balances, err := h.GetTotalBalance(ctx, chainID, nil, ethAddress, true, false)
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		if strings.EqualFold(b.Token.Erc20TokenAddress, tokenAddress) {
			return new(big.Int).Set(b.Balance), nil
		}
	}
	return new(big.Int), nil
}

func waitForPrivateBalanceDelta(
	t *testing.T,
	ctx context.Context,
	h *hinkal.Hinkal,
	chainID int,
	ethAddress, tokenAddress string,
	before, expectedDelta *big.Int,
) *big.Int {
	t.Helper()

	const pollInterval = 5 * time.Second
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()

	var lastBalance = new(big.Int).Set(before)
	var lastErr error
	for {
		balance, err := fetchPrivateBalanceForToken(ctx, h, chainID, ethAddress, tokenAddress)
		if err == nil {
			lastBalance = balance
			lastErr = nil
			delta := new(big.Int).Sub(balance, before)
			if delta.Cmp(expectedDelta) == 0 {
				return balance
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			t.Fatalf("wait for enclave balance mapping: %v", ctx.Err())
		case <-deadline.C:
			delta := new(big.Int).Sub(lastBalance, before)
			t.Fatalf("enclave balance mapping did not reach delta %s within 2m: last delta=%s, last error=%v", expectedDelta, delta, lastErr)
		case <-time.After(pollInterval):
		}
	}
}
