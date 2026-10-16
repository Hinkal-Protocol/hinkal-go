package utils

import (
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	solana "github.com/gagliardetto/solana-go"
)

func HashEthereumAddress(ethereumAddress string) string {
	hash := gethcrypto.Keccak256([]byte(strings.ToLower(ethereumAddress)))
	return "0x" + hex.EncodeToString(hash)
}

var (
	evmAddressPattern   = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{40}$`)
	mixedCaseHexPattern = regexp.MustCompile(`([A-F].*[a-f])|([a-f].*[A-F])`)
)

func IsValidSolanaPublicKey(str string) bool {
	pubkey, err := solana.PublicKeyFromBase58(str)
	if err != nil {
		return false
	}
	// Check that the reconstructed base58 string matches exactly
	return pubkey.String() == str
}

func IsValidTronAddress(address string) bool {
	if len(address) == 42 {
		payload, err := hex.DecodeString(address)
		return err == nil && payload[0] == tronAddressPrefix
	}
	_, err := decodeTronBase58Check(address)
	return err == nil
}

func IsEthersAddress(address string) bool {
	if !evmAddressPattern.MatchString(address) {
		return false
	}
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	return !mixedCaseHexPattern.MatchString(address) || common.HexToAddress(address).Hex() == address
}
