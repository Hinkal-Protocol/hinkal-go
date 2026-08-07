package pretransaction

import (
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func ValidateAndGetChainID(tokens []types.ERC20Token) (int, error) {
	if len(tokens) == 0 {
		return 0, errorhandling.ErrEmptyTokenArray
	}
	firstChainID := tokens[0].ChainID
	for _, token := range tokens {
		if token.ChainID != firstChainID {
			return 0, errorhandling.ErrTokensDifferentChains
		}
	}
	return firstChainID, nil
}
