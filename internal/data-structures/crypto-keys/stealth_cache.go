package cryptokeys

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/cache"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var stealthPairCache = cache.NewAttachableMemoryCacheDevice[cachedStealthPair]()

type cachedStealthPair struct {
	h0 types.JubPoint
	h1 types.JubPoint
}

func stealthCacheKey(s *big.Int, privateKey string) string {
	return s.String() + ":" + privateKey
}

func copyJubPoint(p types.JubPoint) types.JubPoint {
	return types.JubPoint{new(big.Int).Set(p[0]), new(big.Int).Set(p[1])}
}
