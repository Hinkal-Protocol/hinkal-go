package utils

import (
	"encoding/hex"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
)

// GenerateHashFromSeedPhrases mirrors @hinkal/common's generateHashFromSeedPhrases:
// poseidon over the 0x-prefixed hex of the UTF-8 bytes of the space-joined seed phrases.
func GenerateHashFromSeedPhrases(seedPhrases []string) (string, error) {
	seedHex := "0x" + hex.EncodeToString([]byte(strings.Join(seedPhrases, " ")))
	seed, err := ParseBigInt(seedHex)
	if err != nil {
		return "", err
	}
	h, err := crypto.PoseidonBig(seed)
	if err != nil {
		return "", err
	}
	return ToBeHex(h), nil
}
