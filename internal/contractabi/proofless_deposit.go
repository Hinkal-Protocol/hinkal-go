package contractabi

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type abiProoflessFeeStructure struct {
	FeeRecipient common.Address
	FeeToken     common.Address
	FeeAmount    *big.Int
}

func toABIStealthAddressStructures(structures []types.StealthAddressStructure) []abiStealthAddressStructure {
	out := make([]abiStealthAddressStructure, len(structures))
	for i, s := range structures {
		out[i] = abiStealthAddressStructure{
			H0x:            zeroIfNil(s.H0x),
			H0y:            zeroIfNil(s.H0y),
			H1x:            zeroIfNil(s.H1x),
			H1y:            zeroIfNil(s.H1y),
			StealthAddress: zeroIfNil(s.StealthAddress),
		}
	}
	return out
}

func toABIProoflessFeeStructure(feeStructure types.ProoflessFeeStructure) abiProoflessFeeStructure {
	return abiProoflessFeeStructure{
		FeeRecipient: common.HexToAddress(feeStructure.FeeRecipient),
		FeeToken:     common.HexToAddress(feeStructure.FeeToken),
		FeeAmount:    zeroIfNil(feeStructure.FeeAmount),
	}
}

func toBytesSlice(hexValues []string) [][]byte {
	out := make([][]byte, len(hexValues))
	for i, v := range hexValues {
		out[i] = common.FromHex(v)
	}
	return out
}

func PackProoflessDeposit(
	chainID int,
	erc20Addresses []string,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	onChainEncryptedOutputs []string,
	createBlockedUtxos bool,
	feeStructure types.ProoflessFeeStructure,
	orderID string,
) ([]byte, error) {
	hasFee := feeStructure.FeeAmount != nil && feeStructure.FeeAmount.Sign() > 0
	if hasFee {
		wrapperABI, err := HinkalWrapper(chainID)
		if err != nil {
			return nil, err
		}
		return wrapperABI.Pack(
			"prooflessDeposit",
			toAddresses(erc20Addresses),
			amounts,
			toABIStealthAddressStructures(structures),
			toBytesSlice(onChainEncryptedOutputs),
			createBlockedUtxos,
			toABIProoflessFeeStructure(feeStructure),
			orderID,
		)
	}
	hinkalABI, err := Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	return hinkalABI.Pack(
		"prooflessDeposit",
		toAddresses(erc20Addresses),
		amounts,
		toABIStealthAddressStructures(structures),
		toBytesSlice(onChainEncryptedOutputs),
		createBlockedUtxos,
		orderID,
	)
}

func ProoflessDepositTargetAddress(chainID int, feeStructure types.ProoflessFeeStructure) (string, error) {
	if feeStructure.FeeAmount != nil && feeStructure.FeeAmount.Sign() > 0 {
		addr, err := constants.HinkalWrapperAddress(chainID)
		if err != nil {
			return "", err
		}
		if addr == "" {
			return "", errors.New("Hinkal wrapper address not found")
		}
		return addr, nil
	}
	return constants.HinkalAddress(chainID)
}
