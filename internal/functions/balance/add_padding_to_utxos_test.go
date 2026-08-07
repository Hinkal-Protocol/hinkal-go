package balance_test

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestAddPaddingToUtxos_PadsEveryTokenUpToMaxInputWhenAnyTokenNeedsMore(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	tokenA := defaultTestTokenAddress
	tokenB := "0x" + strings.Repeat("22", 20)

	outputs := []*types.EncryptedOutputWithSign{
		sealedOutputForToken(t, uk, tokenA, 50, false),
		sealedOutputForToken(t, uk, tokenB, 10, false),
		sealedOutputForToken(t, uk, tokenB, 20, false),
		sealedOutputForToken(t, uk, tokenB, 30, false),
	}
	fake := newFakeHinkal(uk, outputs)

	got, err := balance.AddPaddingToUtxos(
		context.Background(), fake, constants.ChainIDs.EthMainnet,
		[]string{tokenA, tokenB}, []*big.Int{big.NewInt(0), big.NewInt(0)}, 0, false, false,
	)
	if err != nil {
		t.Fatalf("AddPaddingToUtxos: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (one entry per requested token)", len(got))
	}
	if len(got[0]) != 6 {
		t.Fatalf("tokenA padded length = %d, want 6", len(got[0]))
	}
	if len(got[1]) != 6 {
		t.Fatalf("tokenB padded length = %d, want 6", len(got[1]))
	}

	sumA := big.NewInt(0)
	for _, u := range got[0] {
		sumA.Add(sumA, u.Amount)
	}
	if sumA.Int64() != 50 {
		t.Fatalf("tokenA total = %s, want 50 (padding utxos must be zero-amount)", sumA)
	}

	sumB := big.NewInt(0)
	for _, u := range got[1] {
		sumB.Add(sumB, u.Amount)
	}
	if sumB.Int64() != 60 {
		t.Fatalf("tokenB total = %s, want 60 (10+20+30)", sumB)
	}
}

func TestAddPaddingToUtxos_SkipsPaddingWhenEveryTokenIsAtTheMinimum(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	tokenA := defaultTestTokenAddress
	tokenB := "0x" + strings.Repeat("22", 20)

	outputs := []*types.EncryptedOutputWithSign{
		sealedOutputForToken(t, uk, tokenA, 50, false),
		sealedOutputForToken(t, uk, tokenB, 10, false),
	}
	fake := newFakeHinkal(uk, outputs)

	got, err := balance.AddPaddingToUtxos(
		context.Background(), fake, constants.ChainIDs.EthMainnet,
		[]string{tokenA, tokenB}, []*big.Int{big.NewInt(0), big.NewInt(0)}, 0, false, false,
	)
	if err != nil {
		t.Fatalf("AddPaddingToUtxos: %v", err)
	}
	if len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 2 {
		t.Fatalf("got lengths = [%d %d], want [2 2] (both tokens already at the 2-slot minimum, no extra padding needed)", len(got[0]), len(got[1]))
	}
}
