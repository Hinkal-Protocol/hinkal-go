package solana

import (
	"crypto/sha256"
	"errors"

	"filippo.io/edwards25519"
	"github.com/mr-tron/base58"
)

const pdaMarker = "ProgramDerivedAddress"

var errNoViableBump = errors.New("solana: unable to find a viable program address bump seed")

func IsOnCurve(point []byte) bool {
	if len(point) != 32 {
		return false
	}
	_, err := new(edwards25519.Point).SetBytes(point)
	return err == nil
}

func findProgramAddress(seeds [][]byte, programID []byte) ([]byte, byte, error) {
	for bump := 255; bump >= 0; bump-- {
		h := sha256.New()
		for _, seed := range seeds {
			h.Write(seed)
		}
		h.Write([]byte{byte(bump)})
		h.Write(programID)
		h.Write([]byte(pdaMarker))
		candidate := h.Sum(nil)
		if !IsOnCurve(candidate) {
			return candidate, byte(bump), nil
		}
	}
	return nil, 0, errNoViableBump
}

func GetMerkleAccountPublicKey(programID, originalDeployer string) (string, error) {
	programIDBytes, err := base58.Decode(programID)
	if err != nil {
		return "", err
	}
	originalDeployerBytes, err := base58.Decode(originalDeployer)
	if err != nil {
		return "", err
	}
	seeds := [][]byte{[]byte("hinkal_merkle"), originalDeployerBytes}
	merkleAccount, _, err := findProgramAddress(seeds, programIDBytes)
	if err != nil {
		return "", err
	}
	return base58.Encode(merkleAccount), nil
}
