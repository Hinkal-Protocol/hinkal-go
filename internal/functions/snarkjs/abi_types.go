package snarkjs

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func mustABIType(t string, components []abi.ArgumentMarshaling) abi.Type {
	ty, err := abi.NewType(t, "", components)
	if err != nil {
		panic(fmt.Sprintf("snarkjs: invalid abi type %q: %v", t, err))
	}
	return ty
}

var (
	abiUint16     = mustABIType("uint16", nil)
	abiUint256    = mustABIType("uint256", nil)
	abiAddress    = mustABIType("address", nil)
	abiBytes      = mustABIType("bytes", nil)
	abiBytesArr2  = mustABIType("bytes[][]", nil)
	abiInt256Arr  = mustABIType("int256[]", nil)
	abiBoolArr    = mustABIType("bool[]", nil)
	abiUint256Arr = mustABIType("uint256[]", nil)

	abiHookTuple = mustABIType("tuple", []abi.ArgumentMarshaling{
		{Name: "p0", Type: "address"}, {Name: "p1", Type: "address"},
		{Name: "p2", Type: "bytes"}, {Name: "p3", Type: "bytes"},
	})
	abiFeeTuple = mustABIType("tuple", []abi.ArgumentMarshaling{
		{Name: "p0", Type: "address"}, {Name: "p1", Type: "uint256"}, {Name: "p2", Type: "uint256"},
	})
	abiExternalActionTuple = mustABIType("tuple", []abi.ArgumentMarshaling{
		{Name: "p0", Type: "address"}, {Name: "p1", Type: "uint256"}, {Name: "p2", Type: "bytes"},
	})
)
