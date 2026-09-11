package addresses

import (
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

// Normalize brings an address to the form its chain compares in: Solana addresses are base58, where
// case carries meaning; hex chains compare case-insensitively, and Tron addresses reach us in both
// base58 and hex form, so they normalize to hex first.
func Normalize(chainID int, address string) string {
	if constants.IsSolanaLike(chainID) {
		return address
	}
	if constants.IsTronLike(chainID) {
		if hexAddress, err := utils.AddressToHexFormat(address); err == nil {
			address = hexAddress
		}
	}
	return strings.ToLower(address)
}

func Equal(chainID int, left string, right string) bool {
	return left != "" && right != "" && Normalize(chainID, left) == Normalize(chainID, right)
}
