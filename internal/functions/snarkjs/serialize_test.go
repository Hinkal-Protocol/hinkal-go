package snarkjs_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestSerializeCircomData(t *testing.T) {
	c := types.CircomDataType{
		RootHashHinkal:      big.NewInt(1),
		RootHashHinkalIndex: big.NewInt(2),
		Erc20TokenAddresses: []string{"0xabc"},
		AmountChanges:       []*big.Int{big.NewInt(100), big.NewInt(-50)},
		InputNullifiers:     [][]string{{"11"}},
		OutCommitments:      [][]string{{"22"}},
		StealthAddressStructure: types.StealthAddressStructure{
			H0x: big.NewInt(3), H0y: big.NewInt(4), H1x: big.NewInt(5), H1y: big.NewInt(6), StealthAddress: big.NewInt(7),
		},
		ExternalActionData: types.CircomExternalActionData{
			ExternalAddress:  "0xdef",
			ExternalActionID: big.NewInt(8),
		},
		CalldataHash:    big.NewInt(9),
		EmporiumMessage: big.NewInt(10),
		SlippageValues:  []*big.Int{big.NewInt(-1)},
		FeeStructure: types.FeeStructure{
			FeeToken: "0xfee", FlatFee: big.NewInt(11), VariableRate: big.NewInt(12),
		},
	}

	got := snarkjs.SerializeCircomData(c)

	if *got.RootHashHinkal != "1" {
		t.Fatalf("RootHashHinkal = %v, want %q", got.RootHashHinkal, "1")
	}
	if *got.RootHashHinkalIndex != "2" {
		t.Fatalf("RootHashHinkalIndex = %v, want %q", got.RootHashHinkalIndex, "2")
	}
	wantAmounts := []string{"100", "-50"}
	if len(got.AmountChanges) != len(wantAmounts) || got.AmountChanges[0] != wantAmounts[0] || got.AmountChanges[1] != wantAmounts[1] {
		t.Fatalf("AmountChanges = %v, want %v", got.AmountChanges, wantAmounts)
	}
	if got.StealthAddressStructure.H0x != "3" || got.StealthAddressStructure.StealthAddress != "7" {
		t.Fatalf("StealthAddressStructure = %+v", got.StealthAddressStructure)
	}
	if got.ExternalActionData.ExternalActionID != "8" {
		t.Fatalf("ExternalActionData.ExternalActionID = %q, want %q", got.ExternalActionData.ExternalActionID, "8")
	}
	if got.CalldataHash != "9" || got.EmporiumMessage != "10" {
		t.Fatalf("CalldataHash/EmporiumMessage = %q/%q", got.CalldataHash, got.EmporiumMessage)
	}
	if got.FeeStructure.FlatFee != "11" || got.FeeStructure.VariableRate != "12" {
		t.Fatalf("FeeStructure = %+v", got.FeeStructure)
	}
}

func TestSerializeCircomData_NilRootHashPointersStaySerializedAsNil(t *testing.T) {
	c := types.CircomDataType{
		StealthAddressStructure: types.StealthAddressStructure{
			H0x: big.NewInt(0), H0y: big.NewInt(0), H1x: big.NewInt(0), H1y: big.NewInt(0), StealthAddress: big.NewInt(0),
		},
		ExternalActionData: types.CircomExternalActionData{ExternalActionID: big.NewInt(0)},
		CalldataHash:       big.NewInt(0),
		EmporiumMessage:    big.NewInt(0),
		FeeStructure:       types.FeeStructure{FlatFee: big.NewInt(0), VariableRate: big.NewInt(0)},
	}

	got := snarkjs.SerializeCircomData(c)

	if got.RootHashHinkal != nil {
		t.Fatalf("RootHashHinkal = %v, want nil", *got.RootHashHinkal)
	}
	if got.RootHashHinkalIndex != nil {
		t.Fatalf("RootHashHinkalIndex = %v, want nil", *got.RootHashHinkalIndex)
	}
}
