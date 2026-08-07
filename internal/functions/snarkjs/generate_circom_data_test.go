package snarkjs_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func zeroStealthStructure() types.StealthAddressStructure {
	return types.StealthAddressStructure{
		H0x: big.NewInt(0), H0y: big.NewInt(0), H1x: big.NewInt(0), H1y: big.NewInt(0), StealthAddress: big.NewInt(0),
	}
}

func zeroFeeStructure() types.FeeStructure {
	return types.FeeStructure{FlatFee: big.NewInt(0), VariableRate: big.NewInt(0)}
}

func strPtr(s string) *string { return &s }

func TestGenerateCircomData_UsesOutputUtxoTimeStampWhenPresent(t *testing.T) {
	outputUtxos := [][]*utxo.Utxo{{withAmount(100)}}
	outputUtxos[0][0].TimeStamp = "12345"

	got := snarkjs.GenerateCircomData(
		nil, nil, big.NewInt(1), big.NewInt(0), nil, nil,
		outputUtxos, nil, "", 0, types.ExternalActionZero, "", big.NewInt(0), "", "", big.NewInt(0),
		zeroStealthStructure(), nil, nil, strPtr("9999"), nil, zeroFeeStructure(), "", "",
	)
	if got.TimeStamp != "12345" {
		t.Fatalf("TimeStamp = %q, want the output utxo's timestamp %q (fallback must not override it)", got.TimeStamp, "12345")
	}
}

func TestGenerateCircomData_FallsBackToTimeStampFallbackWhenNoOutputs(t *testing.T) {
	got := snarkjs.GenerateCircomData(
		nil, nil, big.NewInt(1), big.NewInt(0), nil, nil,
		nil, nil, "", 0, types.ExternalActionZero, "", big.NewInt(0), "", "", big.NewInt(0),
		zeroStealthStructure(), nil, nil, strPtr("9999"), nil, zeroFeeStructure(), "", "",
	)
	if got.TimeStamp != "9999" {
		t.Fatalf("TimeStamp = %q, want fallback %q", got.TimeStamp, "9999")
	}
}

func TestGenerateCircomData_DefaultsHookDataAndExternalAddress(t *testing.T) {
	got := snarkjs.GenerateCircomData(
		nil, nil, big.NewInt(1), big.NewInt(0), nil, nil,
		nil, nil, "", 0, types.ExternalActionZero, "", big.NewInt(0), "", "", big.NewInt(0),
		zeroStealthStructure(), nil, nil, nil, nil, zeroFeeStructure(), "", "",
	)
	if got.HookData != types.DefaultHookData() {
		t.Fatalf("HookData = %+v, want the default hook data", got.HookData)
	}
	if got.ExternalActionData.ExternalAddress != constants.ZeroAddress {
		t.Fatalf("ExternalActionData.ExternalAddress = %q, want the zero address", got.ExternalActionData.ExternalAddress)
	}
	if got.OriginalSender != constants.ZeroAddress {
		t.Fatalf("OriginalSender = %q, want the zero address (no relay, no external address)", got.OriginalSender)
	}
}

func TestGenerateCircomData_PreservesExplicitOriginalSender(t *testing.T) {
	got := snarkjs.GenerateCircomData(
		nil, nil, big.NewInt(1), big.NewInt(0), nil, nil,
		nil, nil, "", 0, types.ExternalActionZero, "", big.NewInt(0), "", "", big.NewInt(0),
		zeroStealthStructure(), nil, nil, nil, nil, zeroFeeStructure(), "0xExplicitSender", "",
	)
	if got.OriginalSender != "0xExplicitSender" {
		t.Fatalf("OriginalSender = %q, want the explicitly passed sender to win over the derived default", got.OriginalSender)
	}
}

func TestGenerateCircomData_HashesExternalActionID(t *testing.T) {
	got := snarkjs.GenerateCircomData(
		nil, nil, big.NewInt(1), big.NewInt(0), nil, nil,
		nil, nil, "", 0, types.ExternalActionTransact, "", big.NewInt(0), "", "", big.NewInt(0),
		zeroStealthStructure(), nil, nil, nil, nil, zeroFeeStructure(), "", "",
	)
	want := snarkjs.GetExternalActionIDHash(types.ExternalActionTransact)
	if got.ExternalActionData.ExternalActionID.Cmp(want) != 0 {
		t.Fatalf("ExternalActionData.ExternalActionID = %s, want %s", got.ExternalActionData.ExternalActionID, want)
	}
}
