package utils

import (
	"encoding/hex"
	"strings"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func HashEthereumAddress(ethereumAddress string) string {
	hash := gethcrypto.Keccak256([]byte(strings.ToLower(ethereumAddress)))
	return "0x" + hex.EncodeToString(hash)
}
