package utils_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func TestFormatUnits(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		decimals int
		want     string
	}{
		{"whole token, 18 decimals", 1_000_000_000_000_000_000, 18, "1.0"},
		{"fractional amount trims trailing zeros", 1_500_000, 6, "1.5"},
		{"smaller than one unit gets zero-padded", 5, 6, "0.000005"},
		{"zero decimals is a plain integer", 12345, 0, "12345"},
		{"negative amount keeps the sign", -1_500_000, 6, "-1.5"},
		{"exact zero", 0, 6, "0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.FormatUnits(big.NewInt(tt.value), tt.decimals); got != tt.want {
				t.Fatalf("FormatUnits(%d, %d) = %q, want %q", tt.value, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestParseUnits(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		decimals int
		want     int64
	}{
		{"whole number", "1", 18, 1_000_000_000_000_000_000},
		{"fractional value", "1.5", 6, 1_500_000},
		{"negative value", "-1.5", 6, -1_500_000},
		{"trailing dot with no fraction", "3.", 6, 3_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ParseUnits(tt.value, tt.decimals)
			if err != nil {
				t.Fatalf("ParseUnits(%q, %d): %v", tt.value, tt.decimals, err)
			}
			if got.Cmp(big.NewInt(tt.want)) != 0 {
				t.Fatalf("ParseUnits(%q, %d) = %s, want %d", tt.value, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestParseUnits_RejectsTooManyDecimalPlaces(t *testing.T) {
	if _, err := utils.ParseUnits("1.23456789", 4); err == nil {
		t.Fatal("expected an error when the value has more decimal places than allowed")
	}
}

func TestParseUnits_RejectsMultipleDecimalPoints(t *testing.T) {
	if _, err := utils.ParseUnits("1.2.3", 6); err == nil {
		t.Fatal("expected an error for a malformed decimal value")
	}
}

func TestFormatAndParseUnits_RoundTrip(t *testing.T) {
	amount := big.NewInt(123_456_789)
	formatted := utils.FormatUnits(amount, 6)
	parsed, err := utils.ParseUnits(formatted, 6)
	if err != nil {
		t.Fatalf("ParseUnits(%q): %v", formatted, err)
	}
	if parsed.Cmp(amount) != 0 {
		t.Fatalf("round trip = %s, want %s", parsed, amount)
	}
}
