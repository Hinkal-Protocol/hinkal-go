package tests

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/signers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const tronNileUSDT = "0xECa9bC828A3005B9a3b909f2cc5c2a54794DE05F"

func liveTronPrivateKey(t *testing.T) string {
	t.Helper()
	pk := os.Getenv("HINKAL_TRON_PRIVATE_KEY")
	if pk == "" {
		t.Skip("set HINKAL_TRON_PRIVATE_KEY to run the Tron deposit test")
	}
	return pk
}

func tronDepositToken(t *testing.T) (string, *big.Int) {
	t.Helper()
	amount := big.NewInt(100_000)
	if v := os.Getenv("HINKAL_TRON_AMOUNT"); v != "" {
		parsed, ok := new(big.Int).SetString(v, 10)
		if !ok {
			t.Fatalf("bad HINKAL_TRON_AMOUNT: %q", v)
		}
		amount = parsed
	}
	return tronNileUSDT, amount
}

func newLiveTronHinkal(t *testing.T, ctx context.Context, chainID int) (*hinkal.Hinkal, string) {
	t.Helper()
	signer, err := signers.NewPrivateKeyTronSigner(liveTronPrivateKey(t))
	if err != nil {
		t.Fatalf("tron signer: %v", err)
	}

	adapter := providers.NewTronProviderAdapter(chainID)
	if err := adapter.InitConnector(ctx, signer); err != nil {
		t.Fatalf("tron adapter init: %v", err)
	}

	h := hinkal.NewHinkal(nil)
	if err := h.InitProviderAdapter(ctx, adapter); err != nil {
		t.Fatalf("init provider adapter: %v", err)
	}
	if seed := os.Getenv("HINKAL_SEED_PHRASE"); seed != "" {
		if err := h.InitUserKeysFromSeedPhrases(strings.Fields(seed)); err != nil {
			t.Fatalf("init user keys from seed phrase: %v", err)
		}
	} else if err := h.InitUserKeys(ctx, types.LoginMessageModeProtocol); err != nil {
		t.Fatalf("init user keys: %v", err)
	}
	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}

	tronAddress, err := h.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		t.Fatalf("tron address: %v", err)
	}
	return h, tronAddress
}

func tronPrivateBalance(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, ownerAddress, tokenAddress string) *big.Int {
	t.Helper()
	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}
	balances, err := h.GetTotalBalance(ctx, chainID, nil, ownerAddress, true, false)
	if err != nil {
		t.Fatalf("get total balance: %v", err)
	}
	for _, b := range balances {
		if strings.EqualFold(b.Token.Erc20TokenAddress, tokenAddress) {
			return b.Balance
		}
	}
	return new(big.Int)
}
