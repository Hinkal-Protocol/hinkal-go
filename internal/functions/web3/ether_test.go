package web3_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func token(decimals int) types.ERC20Token {
	return types.ERC20Token{Decimals: decimals}
}

func TestGetAmountInToken(t *testing.T) {
	got := web3.GetAmountInToken(token(6), big.NewInt(1_500_000))
	if got != "1.5" {
		t.Fatalf("GetAmountInToken = %q, want %q", got, "1.5")
	}
}

func TestGetAmountInWei(t *testing.T) {
	tests := []struct {
		name     string
		decimals int
		amount   string
		want     int64
	}{
		{"6-decimal token", 6, "1.5", 1_500_000},
		{"18-decimal token, tiny amount", 18, "0.000001", 1_000_000_000_000},
		{"whole number", 6, "3", 3_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := web3.GetAmountInWei(token(tt.decimals), tt.amount)
			if err != nil {
				t.Fatalf("GetAmountInWei: %v", err)
			}
			if got.Cmp(big.NewInt(tt.want)) != 0 {
				t.Fatalf("GetAmountInWei(%q) = %s, want %d", tt.amount, got, tt.want)
			}
		})
	}
}

func TestGetAmountInWei_RejectsGarbage(t *testing.T) {
	if _, err := web3.GetAmountInWei(token(18), "not-a-number"); err == nil {
		t.Fatal("expected an error for an unparseable amount")
	}
}

func TestGetAmountWithPrecision_TruncatesRatherThanRounds(t *testing.T) {
	balance, _ := new(big.Int).SetString("1234567890123456789", 10)
	got, err := web3.GetAmountWithPrecision(balance, token(18), 6)
	if err != nil {
		t.Fatalf("GetAmountWithPrecision: %v", err)
	}
	if got != "1.234567" {
		t.Fatalf("GetAmountWithPrecision = %q, want %q", got, "1.234567")
	}
}

func TestGetAmountWithPrecision_MatchesFullPrecisionWhenNoTruncationNeeded(t *testing.T) {
	got, err := web3.GetAmountWithPrecision(big.NewInt(1_500_000), token(6), 6)
	if err != nil {
		t.Fatalf("GetAmountWithPrecision: %v", err)
	}
	if got != "1.5" {
		t.Fatalf("GetAmountWithPrecision = %q, want %q", got, "1.5")
	}
}
