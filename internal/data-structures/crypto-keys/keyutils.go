package cryptokeys

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

var (
	circomP                 = new(big.Int).Set(crypto.FieldP)
	circomPHalf             = new(big.Int).Div(crypto.FieldP, big.NewInt(2))
	nullifyingPrivateKeyMod = new(big.Int).Lsh(big.NewInt(1), 250)
)

func isCircomNegative(n *big.Int) bool {
	return n.Cmp(circomPHalf) > 0
}

func getCircomSign(n *big.Int) *big.Int {
	if isCircomNegative(n) {
		return big.NewInt(1)
	}
	return big.NewInt(0)
}

func poseidonHashHex(inputs ...string) (string, error) {
	bigs := make([]*big.Int, len(inputs))
	for i, in := range inputs {
		n, err := utils.ParseBigInt(in)
		if err != nil {
			return "", err
		}
		bigs[i] = n
	}
	h, err := crypto.PoseidonBig(bigs...)
	if err != nil {
		return "", err
	}
	return utils.ToBeHex(h), nil
}

func keccak256Utf8(s string) string {
	return "0x" + hex.EncodeToString(gethcrypto.Keccak256([]byte(s)))
}

func recoverableSignature(signature string) ([]byte, error) {
	sig := common.FromHex(signature)
	if len(sig) != 65 {
		return nil, errors.New("cryptokeys: signature must be 65 bytes")
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	return sig, nil
}
