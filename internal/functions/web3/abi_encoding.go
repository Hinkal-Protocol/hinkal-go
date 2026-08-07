package web3

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func EncodeUint256Array(values []*big.Int) (string, error) {
	uint256Array, err := abi.NewType("uint256[]", "", nil)
	if err != nil {
		return "", err
	}
	packed, err := (abi.Arguments{{Type: uint256Array}}).Pack(values)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(packed), nil
}
