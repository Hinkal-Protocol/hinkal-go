package snarkjs

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type EvmSignedMessageHashParams struct {
	RootHashHinkal      *big.Int
	Erc20TokenAddresses []string
	AmountChanges       []*big.Int
	OutTimeStamp        *big.Int
	InNullifiers        [][]string
	OutCommitments      [][]string
	CalldataHash        *big.Int
	Message             *big.Int
	OutH1Ax             *big.Int
	OutH1Ay             *big.Int
	H0Ax                *big.Int
	H0Ay                *big.Int
}

type SolanaSignedMessageHashParams struct {
	RootHashHinkal               *big.Int
	MintAccountPart1             []*big.Int
	MintAccountPart2             []*big.Int
	AmountChanges                []*big.Int
	OutTimeStamp                 *big.Int
	InNullifiers                 [][]string
	OutCommitments               [][]string
	CalldataHash                 *big.Int
	Message                      *big.Int
	SwapperAccountAdditionalSeed *big.Int
	OutH1Ay                      *big.Int
	H0Ax                         *big.Int
	H0Ay                         *big.Int
}

func addressesToBigInts(addresses []string) ([]*big.Int, error) {
	out := make([]*big.Int, len(addresses))
	for i, a := range addresses {
		n, err := utils.ParseBigInt(a)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func flatToBigInts(values [][]string) ([]*big.Int, error) {
	out := make([]*big.Int, 0)
	for _, inner := range values {
		for _, v := range inner {
			n, err := utils.ParseBigInt(v)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
	}
	return out, nil
}

func appendBytes32(dst []byte, value *big.Int) []byte {
	var out [32]byte
	if value != nil {
		value.FillBytes(out[:])
	}
	return append(dst, out[:]...)
}

func appendLengthPrefixedBytes32Array(dst []byte, values []*big.Int) []byte {
	var length [8]byte
	big.NewInt(int64(len(values))).FillBytes(length[:])
	dst = append(dst, length[:]...)
	for _, value := range values {
		dst = appendBytes32(dst, value)
	}
	return dst
}

func ComputeSignedMessageHashEvm(params EvmSignedMessageHashParams) (*big.Int, error) {
	tokens, err := addressesToBigInts(params.Erc20TokenAddresses)
	if err != nil {
		return nil, err
	}
	nullifiers, err := flatToBigInts(params.InNullifiers)
	if err != nil {
		return nil, err
	}
	commitments, err := flatToBigInts(params.OutCommitments)
	if err != nil {
		return nil, err
	}

	packed, err := (abi.Arguments{
		{Type: abiUint256},    // rootHashHinkal
		{Type: abiUint256Arr}, // erc20TokenAddresses
		{Type: abiUint256Arr}, // amountChanges
		{Type: abiUint256},    // outTimeStamp
		{Type: abiUint256Arr}, // inNullifiers
		{Type: abiUint256Arr}, // outCommitments
		{Type: abiUint256},    // calldataHash
		{Type: abiUint256},    // message
		{Type: abiUint256},    // outH1Ax
		{Type: abiUint256},    // outH1Ay
		{Type: abiUint256},    // H0Ax
		{Type: abiUint256},    // H0Ay
	}).Pack(
		params.RootHashHinkal,
		tokens,
		params.AmountChanges,
		params.OutTimeStamp,
		nullifiers,
		commitments,
		params.CalldataHash,
		params.Message,
		params.OutH1Ax,
		params.OutH1Ay,
		params.H0Ax,
		params.H0Ay,
	)
	if err != nil {
		return nil, fmt.Errorf("signed message hash encode: %w", err)
	}

	h := new(big.Int).SetBytes(gethcrypto.Keccak256(packed))
	return h.Mod(h, circomP), nil
}

func ComputeSignedMessageHashSolana(params SolanaSignedMessageHashParams) (*big.Int, error) {
	nullifiers, err := flatToBigInts(params.InNullifiers)
	if err != nil {
		return nil, err
	}
	commitments, err := flatToBigInts(params.OutCommitments)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, 0)
	bytes = appendBytes32(bytes, params.RootHashHinkal)
	bytes = appendLengthPrefixedBytes32Array(bytes, params.MintAccountPart1)
	bytes = appendLengthPrefixedBytes32Array(bytes, params.MintAccountPart2)
	bytes = appendLengthPrefixedBytes32Array(bytes, params.AmountChanges)
	bytes = appendBytes32(bytes, params.OutTimeStamp)
	bytes = appendLengthPrefixedBytes32Array(bytes, nullifiers)
	bytes = appendLengthPrefixedBytes32Array(bytes, commitments)
	bytes = appendBytes32(bytes, params.CalldataHash)
	bytes = appendBytes32(bytes, params.Message)
	bytes = appendBytes32(bytes, params.SwapperAccountAdditionalSeed)
	bytes = appendBytes32(bytes, params.OutH1Ay)
	bytes = appendBytes32(bytes, params.H0Ax)
	bytes = appendBytes32(bytes, params.H0Ay)

	sum := sha256.Sum256(bytes)
	h := new(big.Int).SetBytes(sum[:])
	return h.Mod(h, circomP), nil
}
