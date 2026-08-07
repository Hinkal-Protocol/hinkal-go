package balance

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestBalanceTokenKey(t *testing.T) {
	t.Run("EVM chain checksums the erc20 address", func(t *testing.T) {
		u := realUtxos(t, 1)[0]
		u.Erc20TokenAddress = "0x0000000000000000000000000000000000000abc"
		got, err := balanceTokenKey(u, constants.ChainIDs.EthMainnet)
		if err != nil {
			t.Fatalf("balanceTokenKey: %v", err)
		}
		if got != common.HexToAddress(u.Erc20TokenAddress).Hex() {
			t.Fatalf("balanceTokenKey = %q, want checksummed %q", got, common.HexToAddress(u.Erc20TokenAddress).Hex())
		}
	})

	t.Run("solana chain returns the mint address as-is", func(t *testing.T) {
		u := realUtxos(t, 1)[0]
		u.MintAddress = "mintAddr123"
		got, err := balanceTokenKey(u, constants.ChainIDs.SolanaMainnet)
		if err != nil {
			t.Fatalf("balanceTokenKey: %v", err)
		}
		if got != "mintAddr123" {
			t.Fatalf("balanceTokenKey = %q, want %q", got, "mintAddr123")
		}
	})
}

func TestIndexOfOutput(t *testing.T) {
	outputs := []*types.EncryptedOutputWithSign{{Value: "a"}, {Value: "b"}, {Value: "c"}}

	if got := indexOfOutput(outputs, "b"); got != 1 {
		t.Fatalf("indexOfOutput(b) = %d, want 1", got)
	}
	if got := indexOfOutput(outputs, "missing"); got != -1 {
		t.Fatalf("indexOfOutput(missing) = %d, want -1", got)
	}
	if got := indexOfOutput(nil, "x"); got != -1 {
		t.Fatalf("indexOfOutput(nil) = %d, want -1", got)
	}
}

func TestFilterSpentUtxos(t *testing.T) {
	utxos := realUtxos(t, 1, 2, 3)
	spentNullifier, err := utxos[1].GetNullifier()
	if err != nil {
		t.Fatal(err)
	}
	nullifiers := map[string]struct{}{spentNullifier: {}}

	got := filterSpentUtxos(utxos, nullifiers)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (one utxo spent)", len(got))
	}
	for _, u := range got {
		if u.Amount.Int64() == 2 {
			t.Fatal("spent utxo (amount=2) was not filtered out")
		}
	}
}

func TestIsIdentityPoint(t *testing.T) {
	tests := []struct {
		name string
		h0   *types.JubPoint
		want bool
	}{
		{"nil point is not identity", nil, false},
		{"identity point (0, 1)", &types.JubPoint{big.NewInt(0), big.NewInt(1)}, true},
		{"non-identity point", &types.JubPoint{big.NewInt(2), big.NewInt(3)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIdentityPoint(tt.h0); got != tt.want {
				t.Fatalf("isIdentityPoint = %v, want %v", got, tt.want)
			}
		})
	}
}
