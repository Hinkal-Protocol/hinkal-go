package snarkjs_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func testUserKeys(t *testing.T) *cryptokeys.UserKeys {
	t.Helper()
	return cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
}

func realUtxo(t *testing.T, uk *cryptokeys.UserKeys, amount int64) *utxo.Utxo {
	t.Helper()
	nullifyingKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	kp, err := uk.GetSpendingKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	u, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            big.NewInt(amount),
		Erc20TokenAddress: constants.ZeroAddress,
		NullifyingKey:     nullifyingKey,
		SpendingPublicKey: []*big.Int{kp.PubSpendingBJJPoint[0], kp.PubSpendingBJJPoint[1]},
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestBuildCommitmentValidationData_HappyPath(t *testing.T) {
	uk := testUserKeys(t)
	spent := realUtxo(t, uk, 100)
	padding := realUtxo(t, uk, 0)

	got, err := snarkjs.BuildCommitmentValidationData(
		constants.ChainIDs.EthMainnet, uk, []string{constants.ZeroAddress}, [][]*utxo.Utxo{{spent, padding}},
	)
	if err != nil {
		t.Fatalf("BuildCommitmentValidationData: %v", err)
	}
	if got == nil {
		t.Fatal("expected a non-nil result")
	}
	if got.VerifierName != "commitmentCalculator1x2" {
		t.Fatalf("VerifierName = %q, want %q", got.VerifierName, "commitmentCalculator1x2")
	}
	if got.InAmounts[0][0] != "100" || got.InAmounts[0][1] != "0" {
		t.Fatalf("InAmounts = %v, want [100 0]", got.InAmounts[0])
	}
	if got.InCommitments[0][1] != "0" {
		t.Fatalf("zero-amount utxo commitment = %q, want the placeholder %q", got.InCommitments[0][1], "0")
	}
	wantCommitment, err := spent.GetCommitment()
	if err != nil {
		t.Fatal(err)
	}
	if got.InCommitments[0][0] == "0" || got.InCommitments[0][0] == "" {
		t.Fatalf("real utxo commitment = %q, want it to reflect %s", got.InCommitments[0][0], wantCommitment)
	}
}

func TestBuildCommitmentValidationData_AllZeroAmountsReturnsNil(t *testing.T) {
	uk := testUserKeys(t)
	padding := realUtxo(t, uk, 0)

	got, err := snarkjs.BuildCommitmentValidationData(
		constants.ChainIDs.EthMainnet, uk, []string{constants.ZeroAddress}, [][]*utxo.Utxo{{padding}},
	)
	if err != nil {
		t.Fatalf("BuildCommitmentValidationData: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for all-zero-amount inputs, got %+v", got)
	}
}

func TestBuildCommitmentValidationData_EmptyInputsReturnNil(t *testing.T) {
	uk := testUserKeys(t)
	spent := realUtxo(t, uk, 100)

	tests := []struct {
		name            string
		erc20Addresses  []string
		inputUtxosArray [][]*utxo.Utxo
	}{
		{"no token addresses", nil, [][]*utxo.Utxo{{spent}}},
		{"no input utxo groups", []string{constants.ZeroAddress}, nil},
		{"empty first group", []string{constants.ZeroAddress}, [][]*utxo.Utxo{{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := snarkjs.BuildCommitmentValidationData(constants.ChainIDs.EthMainnet, uk, tt.erc20Addresses, tt.inputUtxosArray)
			if err != nil {
				t.Fatalf("BuildCommitmentValidationData: %v", err)
			}
			if got != nil {
				t.Fatalf("expected nil, got %+v", got)
			}
		})
	}
}

func TestBuildCommitmentValidationData_RejectsStealthAddressMismatch(t *testing.T) {
	uk := testUserKeys(t)
	tampered := realUtxo(t, uk, 100)
	tampered.StealthAddress = "12345"

	_, err := snarkjs.BuildCommitmentValidationData(
		constants.ChainIDs.EthMainnet, uk, []string{constants.ZeroAddress}, [][]*utxo.Utxo{{tampered}},
	)
	if err == nil || !strings.Contains(err.Error(), "stealth mismatch") {
		t.Fatalf("err = %v, want a stealth mismatch error", err)
	}
}
