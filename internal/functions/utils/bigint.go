package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

func ParseBigInt(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	n := new(big.Int)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if _, ok := n.SetString(s[2:], 16); ok {
			return n, nil
		}
	}
	if _, ok := n.SetString(s, 10); ok {
		return n, nil
	}
	return nil, fmt.Errorf("cannot parse %q as big.Int", s)
}

func RandomBigInt(numBytes int) (*big.Int, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(buf), nil
}

func ToBeHex(n *big.Int) string {
	if n == nil {
		n = new(big.Int)
	}
	s := n.Text(16)
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return "0x" + s
}
