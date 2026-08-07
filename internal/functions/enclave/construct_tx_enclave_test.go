package enclave

import (
	"errors"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestAssertPrepareStuckWithdrawEchoMatches(t *testing.T) {
	payload := types.PrepareStuckWithdrawRequestType{
		ChainID:         "1",
		Erc20Address:    "0xtoken",
		ExternalAddress: "0xrecipient",
		Relay:           "0xrelay",
		FeeStructure: types.FeeStructureJSON{
			FeeToken:     "0xfeetoken",
			FlatFee:      "10",
			VariableRate: "20",
		},
		HashedEthereumAddress: "0xownerhash",
		NullifyingKey:         "private",
		SpendingPublicKey:     [2]string{"private-x", "private-y"},
	}
	matching := types.EchoedPrepareStuckWithdrawRequestType{
		ChainID:               payload.ChainID,
		Erc20Address:          payload.Erc20Address,
		ExternalAddress:       payload.ExternalAddress,
		Relay:                 payload.Relay,
		FeeStructure:          payload.FeeStructure,
		HashedEthereumAddress: payload.HashedEthereumAddress,
	}

	if err := assertPrepareStuckWithdrawEchoMatches(payload, matching); err != nil {
		t.Fatalf("matching echo rejected: %v", err)
	}

	tests := map[string]func(*types.EchoedPrepareStuckWithdrawRequestType){
		"chain":     func(e *types.EchoedPrepareStuckWithdrawRequestType) { e.ChainID = "2" },
		"token":     func(e *types.EchoedPrepareStuckWithdrawRequestType) { e.Erc20Address = "0xother" },
		"recipient": func(e *types.EchoedPrepareStuckWithdrawRequestType) { e.ExternalAddress = "0xattacker" },
		"relay":     func(e *types.EchoedPrepareStuckWithdrawRequestType) { e.Relay = "0xother" },
		"fee": func(e *types.EchoedPrepareStuckWithdrawRequestType) {
			e.FeeStructure.FlatFee = "999"
		},
		"owner": func(e *types.EchoedPrepareStuckWithdrawRequestType) {
			e.HashedEthereumAddress = "0xother"
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			tampered := matching
			mutate(&tampered)
			if err := assertPrepareStuckWithdrawEchoMatches(payload, tampered); !errors.Is(err, errPrepareStuckWithdrawEchoMismatch) {
				t.Fatalf("tampered echo error = %v, want %v", err, errPrepareStuckWithdrawEchoMismatch)
			}
		})
	}
}

func TestAssertPrepareTxEchoMatchesOriginalSender(t *testing.T) {
	payload := types.PrepareTxRequestType{OriginalSender: "0xsender"}
	matching := types.EchoedPrepareTxRequestType{OriginalSender: payload.OriginalSender}

	if err := assertPrepareTxEchoMatches(payload, matching); err != nil {
		t.Fatalf("matching echo rejected: %v", err)
	}

	tampered := matching
	tampered.OriginalSender = "0xattacker"
	if err := assertPrepareTxEchoMatches(payload, tampered); !errors.Is(err, errPrepareTxEchoMismatch) {
		t.Fatalf("tampered originalSender error = %v, want %v", err, errPrepareTxEchoMismatch)
	}
}
