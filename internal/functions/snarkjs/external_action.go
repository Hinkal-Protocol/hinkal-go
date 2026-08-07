package snarkjs

import (
	"math/big"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetExternalActionIDHash(externalActionID types.ExternalActionID) *big.Int {
	if externalActionID == types.ExternalActionZero {
		return big.NewInt(0)
	}
	hash := gethcrypto.Keccak256([]byte(string(externalActionID)))
	n := new(big.Int).SetBytes(hash)
	return n.Mod(n, circomP)
}
