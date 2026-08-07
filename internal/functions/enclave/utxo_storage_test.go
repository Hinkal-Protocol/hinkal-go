package enclave

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestEnclaveUtxoPayloadSerializesLikeConstructorParams(t *testing.T) {
	payload, err := enclaveUtxoPayload(types.UtxoParams{
		Amount:            big.NewInt(123),
		Erc20TokenAddress: constants.ZeroAddress,
		TimeStamp:         "456",
		NullifyingKey:     "0xabc",
		SpendingPublicKey: []*big.Int{big.NewInt(1), big.NewInt(2)},
		Randomization:     big.NewInt(789),
		H0:                &types.JubPoint{big.NewInt(3), big.NewInt(4)},
	}, "0xsender", "0xclaimable")
	if err != nil {
		t.Fatalf("payload: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	for key, want := range map[string]string{
		"amount":             "123",
		"erc20TokenAddress":  constants.ZeroAddress,
		"timeStamp":          "456",
		"nullifyingKey":      "0xabc",
		"randomization":      "789",
		"senderAddress":      "0xsender",
		"claimableSignature": "0xclaimable",
	} {
		if got[key] != want {
			t.Fatalf("%s = %v, want %s", key, got[key], want)
		}
	}
}

func TestDeduplicateUtxosByCommitment(t *testing.T) {
	params := types.UtxoParams{
		Amount:            big.NewInt(1),
		Erc20TokenAddress: constants.ZeroAddress,
		TimeStamp:         "1",
		StealthAddress:    "0x123",
		H0:                &types.JubPoint{big.NewInt(1), big.NewInt(1)},
	}
	items := []types.UtxoConstructorParamsWithSenderAddress{
		{UtxoParams: params, SenderAddress: "0xsender1"},
		{UtxoParams: params, SenderAddress: "0xsender2"},
		{
			UtxoParams: types.UtxoParams{
				Amount:            big.NewInt(2),
				Erc20TokenAddress: constants.ZeroAddress,
				TimeStamp:         "1",
				StealthAddress:    "0x123",
				H0:                &types.JubPoint{big.NewInt(2), big.NewInt(2)},
			},
			SenderAddress: "0xsender3",
		},
	}

	deduped, err := deduplicateUtxosByCommitment(items)
	if err != nil {
		t.Fatalf("dedup: %v", err)
	}
	if len(deduped) != 2 {
		t.Fatalf("len = %d, want 2", len(deduped))
	}
	if deduped[0].SenderAddress != "0xsender1" {
		t.Fatalf("first sender = %s, want first duplicate to be preserved", deduped[0].SenderAddress)
	}
}
