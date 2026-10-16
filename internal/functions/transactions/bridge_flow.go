package transactions

import (
	"math"
	"strconv"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

func getBridgingSlippagePercent(amount string, tokenPriceDollar float64) float64 {
	numericAmount := 0.0
	if trimmed := strings.TrimSpace(amount); trimmed != "" {
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			parsed = math.NaN()
		}
		numericAmount = parsed
	}
	if !math.IsNaN(numericAmount) && !math.IsInf(numericAmount, 0) && tokenPriceDollar > 0 &&
		numericAmount*tokenPriceDollar < constants.LowAmountUSDThreshold {
		return constants.LowAmountBridgingSlippage
	}
	return constants.DefaultBridgingSlippage
}
