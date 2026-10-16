package transactions

import (
	"math"

	tokenchecker "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/token-checker"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// isRecoverableBalance mirrors isRecoverableBalance (recovery-balance.utils.ts).
func isRecoverableBalance(balance types.TokenBalanceWithUsdValue) bool {
	if balance.IsUsdValueLoading {
		return true
	}

	// !balance.usdValue is also true for NaN
	if balance.UsdValue <= 0 || math.IsNaN(balance.UsdValue) {
		return false
	}
	if tokenchecker.IsPotentiallySpamToken(balance.Token) {
		return false
	}
	if tokenchecker.IsPotentiallyVolatile(balance.Token) {
		return false
	}

	return true
}
