package balance

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func testSpendingKey(t *testing.T) (nullifyingKey string, spendingPublicKey []*big.Int) {
	t.Helper()
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	nk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	kp, err := uk.GetSpendingKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return nk, []*big.Int{kp.PubSpendingBJJPoint[0], kp.PubSpendingBJJPoint[1]}
}

func realUtxos(t *testing.T, amounts ...int64) []*utxo.Utxo {
	t.Helper()
	nullifyingKey, spendingPublicKey := testSpendingKey(t)
	out := make([]*utxo.Utxo, len(amounts))
	for i, amount := range amounts {
		u, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            big.NewInt(amount),
			Erc20TokenAddress: constants.ZeroAddress,
			NullifyingKey:     nullifyingKey,
			SpendingPublicKey: spendingPublicKey,
		})
		if err != nil {
			t.Fatal(err)
		}
		out[i] = u
	}
	return out
}

func amounts(utxos []*utxo.Utxo) []int64 {
	out := make([]int64, len(utxos))
	for i, u := range utxos {
		out[i] = u.Amount.Int64()
	}
	return out
}

func TestSortAndPadUtxos_SortsDescendingByAmount(t *testing.T) {
	nullifyingKey, spendingPublicKey := testSpendingKey(t)
	got, err := sortAndPadUtxos(realUtxos(t, 5, 20, 10), 3, false, nullifyingKey, spendingPublicKey, constants.ZeroAddress, "")
	if err != nil {
		t.Fatalf("sortAndPadUtxos: %v", err)
	}
	want := []int64{20, 10, 5}
	if got := amounts(got); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("amounts = %v, want %v", got, want)
	}
}

func TestSortAndPadUtxos_PadsUpToMinInput(t *testing.T) {
	nullifyingKey, spendingPublicKey := testSpendingKey(t)
	got, err := sortAndPadUtxos(realUtxos(t, 5), 3, false, nullifyingKey, spendingPublicKey, constants.ZeroAddress, "")
	if err != nil {
		t.Fatalf("sortAndPadUtxos: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Amount.Int64() != 5 {
		t.Fatalf("real utxo should sort first, got amount %v", got[0].Amount)
	}
	for _, u := range got[1:] {
		if u.Amount.Sign() != 0 {
			t.Fatalf("padding utxo has nonzero amount %v", u.Amount)
		}
	}
}

func TestSortAndPadUtxos_PadsAboveMinInputUpToSix(t *testing.T) {
	nullifyingKey, spendingPublicKey := testSpendingKey(t)
	got, err := sortAndPadUtxos(realUtxos(t, 1, 2, 3), 2, false, nullifyingKey, spendingPublicKey, constants.ZeroAddress, "")
	if err != nil {
		t.Fatalf("sortAndPadUtxos: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("len = %d, want 6 (padded above minInput up to 6)", len(got))
	}
}

func TestSortAndPadUtxos_LeavesSixOrMoreUntouched(t *testing.T) {
	nullifyingKey, spendingPublicKey := testSpendingKey(t)
	input := realUtxos(t, 1, 2, 3, 4, 5, 6, 7, 8)
	got, err := sortAndPadUtxos(input, 2, false, nullifyingKey, spendingPublicKey, constants.ZeroAddress, "")
	if err != nil {
		t.Fatalf("sortAndPadUtxos: %v", err)
	}
	if len(got) != len(input) {
		t.Fatalf("len = %d, want %d (no padding once already >= 6)", len(got), len(input))
	}
}

func TestSortAndPadUtxos_ExactlyMinInputNeedsNoPadding(t *testing.T) {
	nullifyingKey, spendingPublicKey := testSpendingKey(t)
	got, err := sortAndPadUtxos(realUtxos(t, 1, 2), 2, false, nullifyingKey, spendingPublicKey, constants.ZeroAddress, "")
	if err != nil {
		t.Fatalf("sortAndPadUtxos: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (already at minInput)", len(got))
	}
}
