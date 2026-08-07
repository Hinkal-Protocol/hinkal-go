package utils

import (
	"math/big"
	"strconv"
	"strings"
)

func BigintMax(values ...*big.Int) *big.Int {
	if len(values) == 0 {
		return nil
	}
	largest := values[0]
	for _, v := range values[1:] {
		if v.Cmp(largest) > 0 {
			largest = v
		}
	}
	return largest
}

func GetValueFirstNDigit(num float64, numberOfDigits int) string {
	numStr := strconv.FormatFloat(num, 'f', 20, 64)
	indexOfDecimal := strings.IndexByte(numStr, '.')
	if indexOfDecimal == -1 {
		return numStr
	}
	endPosition := indexOfDecimal + numberOfDigits + 1
	if endPosition > len(numStr) {
		endPosition = len(numStr)
	}
	return numStr[:endPosition]
}
