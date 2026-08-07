package snarkjs

import (
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func indexedSignals(count int) [][]int {
	signals := make([][]int, count)
	for i := range signals {
		signals[i] = []int{i}
	}
	return signals
}

func signalIndex(t *testing.T, value []int) int {
	t.Helper()
	if len(value) != 1 {
		t.Fatalf("signal length = %d, want 1", len(value))
	}
	return value[0]
}

func TestConvertSolanaPublicSignals1x2x1Layout(t *testing.T) {
	dimensions := types.DimDataType{TokenNumber: 1, NullifierAmount: 2, OutputAmount: 1}
	count, err := SolanaPublicSignalCount(dimensions)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 17 {
		t.Fatalf("count = %d, want 17", count)
	}

	converted, err := ConvertSolanaPublicSignals(indexedSignals(count), dimensions)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if got := signalIndex(t, converted.OutH1Ax); got != 0 {
		t.Fatalf("OutH1Ax index = %d, want 0", got)
	}
	if got := signalIndex(t, converted.OutH1Ay); got != 1 {
		t.Fatalf("OutH1Ay index = %d, want 1", got)
	}
	if got := signalIndex(t, converted.RootHash); got != 5 {
		t.Fatalf("RootHash index = %d, want 5", got)
	}
	if got := signalIndex(t, converted.SignedMessageHash); got != 6 {
		t.Fatalf("SignedMessageHash index = %d, want 6", got)
	}
	if got := signalIndex(t, converted.AmountChanges[0]); got != 9 {
		t.Fatalf("AmountChanges[0] index = %d, want 9", got)
	}
	if got := signalIndex(t, converted.InNullifiers[0][0]); got != 10 {
		t.Fatalf("InNullifiers[0][0] index = %d, want 10", got)
	}
	if got := signalIndex(t, converted.InNullifiers[0][1]); got != 11 {
		t.Fatalf("InNullifiers[0][1] index = %d, want 11", got)
	}
	if got := signalIndex(t, converted.H0Ax); got != 15 {
		t.Fatalf("H0Ax index = %d, want 15", got)
	}
	if got := signalIndex(t, converted.H0Ay); got != 16 {
		t.Fatalf("H0Ay index = %d, want 16", got)
	}
}

func TestConvertSolanaPublicSignals2x6x1NullifierStart(t *testing.T) {
	dimensions := types.DimDataType{TokenNumber: 2, NullifierAmount: 6, OutputAmount: 1}
	count, err := SolanaPublicSignalCount(dimensions)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	converted, err := ConvertSolanaPublicSignals(indexedSignals(count), dimensions)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if got := signalIndex(t, converted.InNullifiers[0][0]); got != 13 {
		t.Fatalf("InNullifiers[0][0] index = %d, want 13", got)
	}
	if got := signalIndex(t, converted.InNullifiers[1][5]); got != 24 {
		t.Fatalf("InNullifiers[1][5] index = %d, want 24", got)
	}
	if got := signalIndex(t, converted.OutTimestamp); got != 25 {
		t.Fatalf("OutTimestamp index = %d, want 25", got)
	}
}
