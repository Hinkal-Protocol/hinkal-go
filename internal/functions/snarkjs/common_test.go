package snarkjs_test

import (
	"math/big"
	"testing"

	hinkalcrypto "github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
)

func snarkjsFieldP(t *testing.T) *big.Int {
	t.Helper()
	return hinkalcrypto.FieldP
}

func withAmount(amount int64) *utxo.Utxo {
	return &utxo.Utxo{Amount: big.NewInt(amount)}
}

func TestCalcAmountChanges(t *testing.T) {
	tests := []struct {
		name          string
		inputUtxos    [][]*utxo.Utxo
		outputUtxos   [][]*utxo.Utxo
		forCircomData bool
		want          []*big.Int
	}{
		{
			name:        "deposit increases balance, no wraparound",
			inputUtxos:  [][]*utxo.Utxo{{withAmount(100)}},
			outputUtxos: [][]*utxo.Utxo{{withAmount(150)}},
			want:        []*big.Int{big.NewInt(50)},
		},
		{
			name:          "withdrawal wraps negative diff into the circom field",
			inputUtxos:    [][]*utxo.Utxo{{withAmount(150)}},
			outputUtxos:   [][]*utxo.Utxo{{withAmount(100)}},
			forCircomData: false,
			want:          []*big.Int{new(big.Int).Sub(snarkjsFieldP(t), big.NewInt(50))},
		},
		{
			name:          "forCircomData keeps the diff negative",
			inputUtxos:    [][]*utxo.Utxo{{withAmount(150)}},
			outputUtxos:   [][]*utxo.Utxo{{withAmount(100)}},
			forCircomData: true,
			want:          []*big.Int{big.NewInt(-50)},
		},
		{
			name:        "multiple tokens computed independently",
			inputUtxos:  [][]*utxo.Utxo{{withAmount(100)}, {withAmount(200)}},
			outputUtxos: [][]*utxo.Utxo{{withAmount(50)}, {withAmount(300)}},
			want:        []*big.Int{new(big.Int).Sub(snarkjsFieldP(t), big.NewInt(50)), big.NewInt(100)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snarkjs.CalcAmountChanges(tt.inputUtxos, tt.outputUtxos, tt.forCircomData)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Cmp(tt.want[i]) != 0 {
					t.Fatalf("[%d] = %s, want %s", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHasOnlyZeroAmounts(t *testing.T) {
	tests := []struct {
		name       string
		inputUtxos [][]*utxo.Utxo
		want       bool
	}{
		{"empty is not all-zero", [][]*utxo.Utxo{}, false},
		{"all zero amounts", [][]*utxo.Utxo{{withAmount(0), withAmount(0)}}, true},
		{"any nonzero amount disqualifies", [][]*utxo.Utxo{{withAmount(0), withAmount(1)}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := snarkjs.HasOnlyZeroAmounts(tt.inputUtxos); got != tt.want {
				t.Fatalf("HasOnlyZeroAmounts = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSlippageValues(t *testing.T) {
	amountChanges := []*big.Int{big.NewInt(50), big.NewInt(-30), big.NewInt(0)}
	want := []int64{0, -30, 0}

	got := snarkjs.GetSlippageValues(amountChanges)
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Cmp(big.NewInt(w)) != 0 {
			t.Fatalf("[%d] = %s, want %d", i, got[i], w)
		}
	}
}

func TestCalcPublicSignalCount(t *testing.T) {
	tests := []struct {
		name        string
		verifier    string
		tokens      []string
		amounts     []*big.Int
		nullifiers  [][]string
		commitments [][]string
		want        int
	}{
		{
			name:     "min0 verifier is a fixed size regardless of inputs",
			verifier: "mainEVMCircuitMin0Something",
			tokens:   []string{"a", "b", "c"},
			want:     3,
		},
		{
			name:        "signal count grows with tokens and flattened nullifiers/commitments",
			verifier:    "mainEVMCircuit2x2x1",
			tokens:      []string{"a", "b"},
			amounts:     []*big.Int{big.NewInt(1), big.NewInt(2)},
			nullifiers:  [][]string{{"n1", "n2"}, {"n3"}},
			commitments: [][]string{{"c1"}},
			want:        18,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snarkjs.CalcPublicSignalCount(tt.verifier, tt.tokens, tt.amounts, tt.nullifiers, tt.commitments)
			if got != tt.want {
				t.Fatalf("CalcPublicSignalCount = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetZkProofVerifierName(t *testing.T) {
	if got := snarkjs.GetZkProofVerifierName([][]*utxo.Utxo{{withAmount(0)}}, [][]*utxo.Utxo{}); got != "mainEVMCircuitMin0" {
		t.Fatalf("no outputs = %q, want mainEVMCircuitMin0", got)
	}

	inputUtxos := [][]*utxo.Utxo{{withAmount(0), withAmount(0)}, {withAmount(0), withAmount(0)}}
	outputUtxos := [][]*utxo.Utxo{{withAmount(0), withAmount(0), withAmount(0)}}
	if got := snarkjs.GetZkProofVerifierName(inputUtxos, outputUtxos); got != "mainEVMCircuit2x2x3" {
		t.Fatalf("verifier name = %q, want mainEVMCircuit2x2x3", got)
	}
}
