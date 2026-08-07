package snarkjs

import (
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SolanaPublicSignals struct {
	OutH1Ax                      []int
	OutH1Ay                      []int
	OutStealthAddress            []int
	Message                      []int
	SwapperAccountAdditionalSeed []int
	RootHash                     []int
	SignedMessageHash            []int
	MintAccountPart1             [][]int
	MintAccountPart2             [][]int
	AmountChanges                [][]int
	InNullifiers                 [][][]int
	OutTimestamp                 []int
	OutCommitments               [][][]int
	CalldataHash                 []int
	H0Ax                         []int
	H0Ay                         []int
}

func SolanaPublicSignalCount(dimensions types.DimDataType) (int, error) {
	tokenCount := dimensions.TokenNumber
	inputCount := dimensions.NullifierAmount
	outputCount := dimensions.OutputAmount
	if tokenCount < 0 || inputCount < 0 || outputCount < 0 {
		return 0, fmt.Errorf("solana public signals: negative dimensions: %+v", dimensions)
	}

	return 7 + 3*tokenCount + tokenCount*inputCount + 1 + tokenCount*outputCount + 1 + 2, nil
}

func ConvertSolanaPublicSignals(
	publicInputsArr [][]int,
	dimensions types.DimDataType,
) (SolanaPublicSignals, error) {
	expected, err := SolanaPublicSignalCount(dimensions)
	if err != nil {
		return SolanaPublicSignals{}, err
	}
	if len(publicInputsArr) != expected {
		return SolanaPublicSignals{}, fmt.Errorf(
			"solana public signals: invalid length %d, expected %d",
			len(publicInputsArr),
			expected,
		)
	}

	tokenCount := dimensions.TokenNumber
	inputCount := dimensions.NullifierAmount
	outputCount := dimensions.OutputAmount
	index := 0
	take := func() []int {
		value := publicInputsArr[index]
		index++
		return value
	}

	out := SolanaPublicSignals{
		OutH1Ax:                      take(),
		OutH1Ay:                      take(),
		OutStealthAddress:            take(),
		Message:                      take(),
		SwapperAccountAdditionalSeed: take(),
		RootHash:                     take(),
		SignedMessageHash:            take(),
		MintAccountPart1:             make([][]int, tokenCount),
		MintAccountPart2:             make([][]int, tokenCount),
		AmountChanges:                make([][]int, tokenCount),
		InNullifiers:                 make([][][]int, tokenCount),
		OutCommitments:               make([][][]int, tokenCount),
	}

	for i := 0; i < tokenCount; i++ {
		out.MintAccountPart1[i] = take()
	}
	for i := 0; i < tokenCount; i++ {
		out.MintAccountPart2[i] = take()
	}
	for i := 0; i < tokenCount; i++ {
		out.AmountChanges[i] = take()
	}
	for tokenIndex := 0; tokenIndex < tokenCount; tokenIndex++ {
		out.InNullifiers[tokenIndex] = make([][]int, inputCount)
		for inputIndex := 0; inputIndex < inputCount; inputIndex++ {
			out.InNullifiers[tokenIndex][inputIndex] = take()
		}
	}

	out.OutTimestamp = take()

	for tokenIndex := 0; tokenIndex < tokenCount; tokenIndex++ {
		out.OutCommitments[tokenIndex] = make([][]int, outputCount)
		for outputIndex := 0; outputIndex < outputCount; outputIndex++ {
			out.OutCommitments[tokenIndex][outputIndex] = take()
		}
	}

	out.CalldataHash = take()
	out.H0Ax = take()
	out.H0Ay = take()

	return out, nil
}
