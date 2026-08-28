package snarkjs

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func DeserializeCircomData(d types.CircomDataJSONType) (types.CircomDataType, error) {
	amountChanges, err := parseBigInts(d.AmountChanges)
	if err != nil {
		return types.CircomDataType{}, err
	}
	slippageValues, err := parseBigInts(d.SlippageValues)
	if err != nil {
		return types.CircomDataType{}, err
	}
	calldataHash, err := utils.ParseBigInt(d.CalldataHash)
	if err != nil {
		return types.CircomDataType{}, err
	}
	emporiumMessage, err := utils.ParseBigInt(d.EmporiumMessage)
	if err != nil {
		return types.CircomDataType{}, err
	}
	externalActionID, err := utils.ParseBigInt(d.ExternalActionData.ExternalActionID)
	if err != nil {
		return types.CircomDataType{}, err
	}
	stealth, err := deserializeStealthAddressStructure(d.StealthAddressStructure)
	if err != nil {
		return types.CircomDataType{}, err
	}
	feeStructure, err := deserializeFeeStructure(d.FeeStructure)
	if err != nil {
		return types.CircomDataType{}, err
	}

	var newRootHash, insertedLeafIndex *big.Int
	if d.NewRootHash != nil {
		parsed, err := utils.ParseBigInt(*d.NewRootHash)
		if err != nil {
			return types.CircomDataType{}, err
		}
		newRootHash = parsed
	}
	if d.InsertedLeafIndex != nil {
		parsed, err := utils.ParseBigInt(*d.InsertedLeafIndex)
		if err != nil {
			return types.CircomDataType{}, err
		}
		insertedLeafIndex = parsed
	}

	result := types.CircomDataType{
		NewRootHash:             newRootHash,
		InsertedLeafIndex:       insertedLeafIndex,
		CreateBlockedUtxos:      d.CreateBlockedUtxos,
		Erc20TokenAddresses:     d.Erc20TokenAddresses,
		AmountChanges:           amountChanges,
		InputNullifiers:         d.InputNullifiers,
		OutCommitments:          d.OutCommitments,
		EncryptedOutputs:        d.EncryptedOutputs,
		OnChainEncryptedOutput:  d.OnChainEncryptedOutput,
		TimeStamp:               d.TimeStamp,
		StealthAddressStructure: stealth,
		Relay:                   d.Relay,
		ExternalActionData: types.CircomExternalActionData{
			ExternalAddress:        d.ExternalActionData.ExternalAddress,
			ExternalActionID:       externalActionID,
			ExternalActionMetadata: d.ExternalActionData.ExternalActionMetadata,
		},
		HookData:          d.HookData,
		CalldataHash:      calldataHash,
		EmporiumMessage:   emporiumMessage,
		PublicSignalCount: d.PublicSignalCount,
		OnChainCreation:   d.OnChainCreation,
		SlippageValues:    slippageValues,
		FeeStructure:      feeStructure,
		OriginalSender:    d.OriginalSender,
		ExtraData:         d.ExtraData,
	}
	if d.RootHashHinkal != nil {
		v, err := utils.ParseBigInt(*d.RootHashHinkal)
		if err != nil {
			return types.CircomDataType{}, err
		}
		result.RootHashHinkal = v
	}
	if d.RootHashHinkalIndex != nil {
		v, err := utils.ParseBigInt(*d.RootHashHinkalIndex)
		if err != nil {
			return types.CircomDataType{}, err
		}
		result.RootHashHinkalIndex = v
	}
	return result, nil
}

func deserializeStealthAddressStructure(s types.StealthAddressStructureJSON) (types.StealthAddressStructure, error) {
	h0x, err := utils.ParseBigInt(s.H0x)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h0y, err := utils.ParseBigInt(s.H0y)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h1x, err := utils.ParseBigInt(s.H1x)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h1y, err := utils.ParseBigInt(s.H1y)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	stealthAddress, err := utils.ParseBigInt(s.StealthAddress)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	return types.StealthAddressStructure{H0x: h0x, H0y: h0y, H1x: h1x, H1y: h1y, StealthAddress: stealthAddress}, nil
}

func deserializeFeeStructure(f types.FeeStructureJSON) (types.FeeStructure, error) {
	flatFee, err := utils.ParseBigInt(f.FlatFee)
	if err != nil {
		return types.FeeStructure{}, err
	}
	variableRate, err := utils.ParseBigInt(f.VariableRate)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return types.FeeStructure{FeeToken: f.FeeToken, FlatFee: flatFee, VariableRate: variableRate}, nil
}

func parseBigInts(values []string) ([]*big.Int, error) {
	out := make([]*big.Int, len(values))
	for i, v := range values {
		n, err := utils.ParseBigInt(v)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}
