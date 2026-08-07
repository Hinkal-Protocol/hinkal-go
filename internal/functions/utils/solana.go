package utils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

const (
	solanaOnChainUtxoPrefix = "solana-on-chain-utxo:"
	solanaOnChainFields     = 9
)

func AdvancedToBigInt(arg string) (*big.Int, error) {
	if bytes, ok := tryParseByteArray(arg); ok {
		if len(bytes) != 32 {
			return nil, fmt.Errorf("advancedToBigInt: expected 32 bytes, got %d", len(bytes))
		}
		return new(big.Int).SetBytes(bytes), nil
	}
	return ParseBigInt(arg)
}

func ParseByteArray(arg string) ([]byte, error) {
	bytes, ok := tryParseByteArray(arg)
	if !ok {
		return nil, fmt.Errorf("cannot parse %q as byte array", arg)
	}
	return bytes, nil
}

func ParseByteMatrix(arg string) ([][]byte, error) {
	var rows [][]int
	if err := json.Unmarshal([]byte(arg), &rows); err != nil {
		return nil, fmt.Errorf("cannot parse %q as byte matrix: %w", arg, err)
	}
	out := make([][]byte, len(rows))
	for i, row := range rows {
		b := make([]byte, len(row))
		for j, n := range row {
			b[j] = byte(n)
		}
		out[i] = b
	}
	return out, nil
}

func EncodeSolanaOnChainUtxo(onChainData [][]byte) (string, error) {
	if len(onChainData) != solanaOnChainFields {
		return "", fmt.Errorf("expected %d on-chain fields, received %d", solanaOnChainFields, len(onChainData))
	}
	var sb strings.Builder
	for i, field := range onChainData {
		if len(field) != 32 {
			return "", fmt.Errorf("expected bytes32 length 32 at position %d, got %d", i, len(field))
		}
		sb.WriteString(hex.EncodeToString(field))
	}
	return solanaOnChainUtxoPrefix + "0x" + sb.String(), nil
}

func IsSolanaOnChainUtxo(encodedOutput string) bool {
	return strings.HasPrefix(encodedOutput, solanaOnChainUtxoPrefix)
}

func tryParseByteArray(arg string) ([]byte, bool) {
	var nums []int
	if err := json.Unmarshal([]byte(arg), &nums); err != nil {
		return nil, false
	}
	out := make([]byte, len(nums))
	for i, n := range nums {
		out[i] = byte(n)
	}
	return out, true
}
