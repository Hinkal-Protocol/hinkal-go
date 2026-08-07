package web3

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestBuildAnchorStealthAddressStructureUsesSingleFieldSpelling(t *testing.T) {
	structure := BuildAnchorStealthAddressStructure(types.StealthAddressStructure{
		H0x:            big.NewInt(1),
		H0y:            big.NewInt(2),
		H1x:            big.NewInt(3),
		H1y:            big.NewInt(4),
		StealthAddress: big.NewInt(5),
	})

	if structure.H0X[31] != 1 {
		t.Fatalf("H0X last byte = %d, want 1", structure.H0X[31])
	}
	if structure.H0Y[31] != 2 {
		t.Fatalf("H0Y last byte = %d, want 2", structure.H0Y[31])
	}
	if structure.H1X[31] != 3 {
		t.Fatalf("H1X last byte = %d, want 3", structure.H1X[31])
	}
	if structure.H1Y[31] != 4 {
		t.Fatalf("H1Y last byte = %d, want 4", structure.H1Y[31])
	}
	if structure.StealthAddress[31] != 5 {
		t.Fatalf("StealthAddress last byte = %d, want 5", structure.StealthAddress[31])
	}
}
