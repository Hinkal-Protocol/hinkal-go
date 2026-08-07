package utils

import (
	"fmt"
	"math/big"
	"strings"
)

func FormatUnits(value *big.Int, decimals int) string {
	if value == nil {
		value = new(big.Int)
	}
	neg := value.Sign() < 0
	digits := new(big.Int).Abs(value).String()
	if decimals == 0 {
		if neg {
			return "-" + digits
		}
		return digits
	}
	if len(digits) <= decimals {
		digits = strings.Repeat("0", decimals+1-len(digits)) + digits
	}
	intPart := digits[:len(digits)-decimals]
	fracPart := strings.TrimRight(digits[len(digits)-decimals:], "0")
	if fracPart == "" {
		fracPart = "0"
	}
	result := intPart + "." + fracPart
	if neg {
		result = "-" + result
	}
	return result
}

func ParseUnits(value string, decimals int) (*big.Int, error) {
	value = strings.TrimSpace(value)
	neg := strings.HasPrefix(value, "-")
	if neg {
		value = value[1:]
	}
	parts := strings.SplitN(value, ".", 2)
	if len(strings.Split(value, ".")) > 2 {
		return nil, fmt.Errorf("invalid decimal value %q", value)
	}
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if len(fracPart) > decimals {
		return nil, fmt.Errorf("too many decimals in %q", value)
	}
	combined := intPart + fracPart + strings.Repeat("0", decimals-len(fracPart))
	if combined == "" {
		combined = "0"
	}
	n, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("cannot parse %q as decimal", value)
	}
	if neg {
		n.Neg(n)
	}
	return n, nil
}
