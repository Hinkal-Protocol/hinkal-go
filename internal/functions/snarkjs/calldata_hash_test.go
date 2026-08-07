package snarkjs_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestGetOriginalSender(t *testing.T) {
	tests := []struct {
		name            string
		externalAddress string
		relay           string
		want            string
	}{
		{"no relay, real external address", "0xabc", constants.ZeroAddress, "0xabc"},
		{"no relay, no external address", "", constants.ZeroAddress, constants.ZeroAddress},
		{"relay set forces zero address", "0xabc", "0xdef", constants.ZeroAddress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := snarkjs.GetOriginalSender(tt.externalAddress, tt.relay); got != tt.want {
				t.Fatalf("GetOriginalSender = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetExternalActionIDHash(t *testing.T) {
	tests := []struct {
		id   types.ExternalActionID
		want string
	}{
		{types.ExternalActionZero, "0"},
		{types.ExternalActionTransact, "1547064758137929605017263462742697347676465663403919601316101326395400313841"},
	}
	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			if got := snarkjs.GetExternalActionIDHash(tt.id); got.String() != tt.want {
				t.Fatalf("GetExternalActionIDHash(%q) = %s, want %s", tt.id, got, tt.want)
			}
		})
	}
}

func callDataHash(t *testing.T, emporiumMessage int64) *big.Int {
	t.Helper()
	h, err := snarkjs.CreateCallDataHash(
		18,
		"0x0000000000000000000000000000000000000001",
		"0x0000000000000000000000000000000000000002",
		types.ExternalActionTransact,
		big.NewInt(emporiumMessage),
		"0x",
		[][]string{{"0x1234"}},
		"0x5678",
		nil,
		[]*big.Int{big.NewInt(0)},
		[]bool{false},
		types.FeeStructure{FeeToken: constants.ZeroAddress, FlatFee: big.NewInt(100), VariableRate: big.NewInt(0)},
		"",
		"0x",
	)
	if err != nil {
		t.Fatalf("CreateCallDataHash: %v", err)
	}
	return h
}

func TestCreateCallDataHash_MatchesKnownGoodValue(t *testing.T) {
	const want = "9480818769709171932527068362446148473629107180877127231948006747334702287962"
	if got := callDataHash(t, 42); got.String() != want {
		t.Fatalf("CreateCallDataHash = %s, want %s", got, want)
	}
}

func TestCreateCallDataHash_IsDeterministic(t *testing.T) {
	if callDataHash(t, 42).Cmp(callDataHash(t, 42)) != 0 {
		t.Fatal("same inputs produced different hashes")
	}
}

func TestCreateCallDataHash_ChangesWithInput(t *testing.T) {
	if callDataHash(t, 42).Cmp(callDataHash(t, 43)) == 0 {
		t.Fatal("changing emporiumMessage did not change the calldata hash")
	}
}
