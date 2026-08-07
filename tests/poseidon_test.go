package tests

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
)

func TestPoseidonParity(t *testing.T) {
	h1, err := crypto.PoseidonBig(big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "poseidon(1)", h1.String(),
		"18586133768512220936620570745912940619677854269274689475585506675881198879027")

	h2, err := crypto.PoseidonBig(big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "poseidon(1,2)", h2.String(),
		"7853200120776062878684798364095072458815029376092732009249414926327459813530")
}
