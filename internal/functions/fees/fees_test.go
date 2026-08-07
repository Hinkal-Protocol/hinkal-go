package fees

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func feeStructure(flatFee, variableRateBps int64) types.FeeStructure {
	return types.FeeStructure{
		FlatFee:      big.NewInt(flatFee),
		VariableRate: big.NewInt(variableRateBps),
	}
}

func TestCalculateTotalFee(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		fee    types.FeeStructure
		want   int64
	}{
		{"flat fee only", 100000, feeStructure(750, 0), 750},
		{"flat + variable", 100000, feeStructure(1000, 250), 3500},
		{"zero amount", 0, feeStructure(500, 250), 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTotalFee(big.NewInt(tt.amount), tt.fee)
			if got.Cmp(big.NewInt(tt.want)) != 0 {
				t.Fatalf("CalculateTotalFee(%d) = %s, want %d", tt.amount, got, tt.want)
			}
		})
	}
}

func TestCalculateWithdrawalAmount(t *testing.T) {
	tests := []struct {
		name          string
		amountWithFee int64 // negative: spend from the ledger
		fee           types.FeeStructure
		want          int64
	}{
		{"zero variable rate", -1500, feeStructure(500, 0), 1000},
		{"flat + variable, exact division", -5100, feeStructure(1000, 250), 4000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateWithdrawalAmount(big.NewInt(tt.amountWithFee), tt.fee)
			if got.Cmp(big.NewInt(tt.want)) != 0 {
				t.Fatalf("CalculateWithdrawalAmount(%d) = %s, want %d", tt.amountWithFee, got, tt.want)
			}
		})
	}
}

func TestCalculateGrossAmountWithFee(t *testing.T) {
	tests := []struct {
		name      string
		netAmount int64
		fee       types.FeeStructure
		want      int64
	}{
		{"zero variable rate", 1000, feeStructure(500, 0), 1500},
		{"flat + variable, exact division", 900, feeStructure(0, 1000), 1000},
		{"rounds up so recipient is never short-changed", 901, feeStructure(0, 1000), 1002},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateGrossAmountWithFee(big.NewInt(tt.netAmount), tt.fee)
			if got.Cmp(big.NewInt(tt.want)) != 0 {
				t.Fatalf("CalculateGrossAmountWithFee(%d) = %s, want %d", tt.netAmount, got, tt.want)
			}
		})
	}
}

func TestGetGasTokenSymbols_AllChains(t *testing.T) {
	got, err := GetGasTokenSymbols(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetGasTokenSymbols: %v", err)
	}
	if !reflect.DeepEqual(got, constants.ExtendedNonNativeGasCostTokenSymbolOptions) {
		t.Fatalf("GetGasTokenSymbols(0) = %v, want %v", got, constants.ExtendedNonNativeGasCostTokenSymbolOptions)
	}
}
