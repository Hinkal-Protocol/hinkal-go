package utils

import "math/big"

var (
	bit255     = new(big.Int).Lsh(big.NewInt(1), 255)
	bit255Mask = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1))
)

// Add the highest bit to a number
func AddHighestBitToNumber(value *big.Int) *big.Int {
	return new(big.Int).Or(value, bit255)
}

func IntMatrixToByteMatrix(rows [][]int) [][]byte {
	out := make([][]byte, len(rows))
	for i, row := range rows {
		out[i] = make([]byte, len(row))
		for j, value := range row {
			out[i][j] = byte(value)
		}
	}
	return out
}

// Take off the highest bit from a number
func TakeOffHighestBit(value *big.Int) *big.Int {
	return new(big.Int).And(value, bit255Mask)
}

// Extract the highest bit from a number
func ExtractHighestBit(value *big.Int) *big.Int {
	return new(big.Int).And(new(big.Int).Rsh(value, 255), big.NewInt(1))
}
