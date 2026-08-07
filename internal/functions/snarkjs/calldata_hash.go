package snarkjs

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type hookTuple struct {
	P0 common.Address
	P1 common.Address
	P2 []byte
	P3 []byte
}

type feeTuple struct {
	P0 common.Address
	P1 *big.Int
	P2 *big.Int
}

type externalActionTuple struct {
	P0 common.Address
	P1 *big.Int
	P2 []byte
}

func hookDataValues(hookData *types.HookDataType) hookTuple {
	if hookData == nil {
		d := types.DefaultHookData()
		return hookTuple{
			P0: common.HexToAddress(d.PreHookContract),
			P1: common.HexToAddress(d.PostHookContract),
			P2: common.FromHex(d.PreHookMetadata),
			P3: common.FromHex(d.PostHookMetadata),
		}
	}
	return hookTuple{
		P0: common.HexToAddress(hookData.PreHookContract),
		P1: common.HexToAddress(hookData.PostHookContract),
		P2: common.FromHex(hookData.PreHookMetadata),
		P3: common.FromHex(hookData.PostHookMetadata),
	}
}

func encryptedOutputsToBytes(encryptedOutputs [][]string) [][][]byte {
	out := make([][][]byte, len(encryptedOutputs))
	for i, inner := range encryptedOutputs {
		out[i] = make([][]byte, len(inner))
		for j, v := range inner {
			out[i][j] = common.FromHex(v)
		}
	}
	return out
}

func CreateCallDataHash(
	chainID int,
	publicSignalCount int,
	relay string,
	externalAddress string,
	externalActionID types.ExternalActionID,
	emporiumMessage *big.Int,
	externalActionMetadata string,
	encryptedOutputs [][]string,
	onChainEncryptedOutput string,
	hookData *types.HookDataType,
	slippageValues []*big.Int,
	onChainCreation []bool,
	feeStructure types.FeeStructure,
	originalSender string,
	extraData string,
) (*big.Int, error) {
	if originalSender == "" {
		originalSender = GetOriginalSender(externalAddress, relay)
	}
	if externalAddress == "" {
		externalAddress = constants.ZeroAddress
	}

	isTron := constants.IsTronLike(chainID)

	args1 := abi.Arguments{{Type: abiUint16}, {Type: abiAddress}, {Type: abiUint256}, {Type: abiExternalActionTuple}}
	values1 := []any{
		uint16(publicSignalCount),
		common.HexToAddress(relay),
		emporiumMessage,
		externalActionTuple{
			P0: common.HexToAddress(externalAddress),
			P1: GetExternalActionIDHash(externalActionID),
			P2: common.FromHex(externalActionMetadata),
		},
	}
	if !isTron {
		args1 = append(args1, abi.Argument{Type: abiInt256Arr})
		values1 = append(values1, slippageValues)
	}

	encodedValues1, err := args1.Pack(values1...)
	if err != nil {
		return nil, fmt.Errorf("calldata encode 1: %w", err)
	}

	feeStructureValue := feeTuple{P0: common.HexToAddress(feeStructure.FeeToken), P1: feeStructure.FlatFee, P2: feeStructure.VariableRate}

	args2 := abi.Arguments{{Type: abiHookTuple}, {Type: abiBytesArr2}}
	values2 := []any{hookDataValues(hookData), encryptedOutputsToBytes(encryptedOutputs)}
	if isTron {
		args2 = append(args2, abi.Argument{Type: abiFeeTuple}, abi.Argument{Type: abiInt256Arr})
		values2 = append(values2, feeStructureValue, slippageValues)
	} else {
		args2 = append(args2, abi.Argument{Type: abiBytes}, abi.Argument{Type: abiFeeTuple})
		values2 = append(values2, common.FromHex(onChainEncryptedOutput), feeStructureValue)
	}
	args2 = append(args2, abi.Argument{Type: abiBoolArr}, abi.Argument{Type: abiAddress}, abi.Argument{Type: abiBytes})
	values2 = append(values2, onChainCreation, common.HexToAddress(originalSender), common.FromHex(extraData))

	encodedValues2, err := args2.Pack(values2...)
	if err != nil {
		return nil, fmt.Errorf("calldata encode 2: %w", err)
	}

	calldataHash1 := new(big.Int).SetBytes(gethcrypto.Keccak256(encodedValues1))
	calldataHash2 := new(big.Int).SetBytes(gethcrypto.Keccak256(encodedValues2))

	encodedValues, err := (abi.Arguments{{Type: abiUint256}, {Type: abiUint256}}).Pack(calldataHash1, calldataHash2)
	if err != nil {
		return nil, fmt.Errorf("calldata encode final: %w", err)
	}

	calldataHash := new(big.Int).SetBytes(gethcrypto.Keccak256(encodedValues))
	return calldataHash.Mod(calldataHash, circomP), nil
}
