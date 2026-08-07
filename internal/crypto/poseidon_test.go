package crypto_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
)

func TestPoseidonBig_ReducesInputsModuloTheField(t *testing.T) {
	// A circuit only ever sees values mod FieldP, so anything the Go side hashes
	// must reduce inputs the same way or a proof built from an out-of-range
	// value would silently disagree with what the circuit computes.
	base := big.NewInt(5)
	overField := new(big.Int).Add(crypto.FieldP, base)

	got, err := crypto.PoseidonBig(overField)
	if err != nil {
		t.Fatalf("PoseidonBig(FieldP+5): %v", err)
	}
	want, err := crypto.PoseidonBig(base)
	if err != nil {
		t.Fatalf("PoseidonBig(5): %v", err)
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("PoseidonBig(FieldP+5) = %s, want it to equal PoseidonBig(5) = %s", got, want)
	}
}

func TestPoseidonBig_IsDeterministicAndInputSensitive(t *testing.T) {
	a, err := crypto.PoseidonBig(big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	b, err := crypto.PoseidonBig(big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	if a.Cmp(b) != 0 {
		t.Fatal("same inputs produced different hashes")
	}

	c, err := crypto.PoseidonBig(big.NewInt(1), big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	if a.Cmp(c) == 0 {
		t.Fatal("different inputs produced the same hash")
	}
}

func TestPoseidonHashFunc_AgreesWithPoseidonBigForTwoInputs(t *testing.T) {
	// The merkle tree combines nodes via PoseidonHashFunc while commitments/nullifiers
	// are computed via PoseidonBig elsewhere; if these two ever diverged for the same
	// already-reduced inputs, merkle proofs would stop matching commitment hashes.
	a, b := big.NewInt(123), big.NewInt(456)

	viaHashFunc := crypto.PoseidonHashFunc(a, b)
	viaBig, err := crypto.PoseidonBig(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if viaHashFunc.Cmp(viaBig) != 0 {
		t.Fatalf("PoseidonHashFunc = %s, PoseidonBig = %s, want them to agree", viaHashFunc, viaBig)
	}
}
