package snarkjs

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func bigIntPtrString(n *big.Int) *string {
	if n == nil {
		return nil
	}
	s := n.String()
	return &s
}

func bigIntSliceStrings(values []*big.Int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

func SerializeCircomData(c types.CircomDataType) types.CircomDataJSONType {
	return types.CircomDataJSONType{
		RootHashHinkal:         bigIntPtrString(c.RootHashHinkal),
		RootHashHinkalIndex:    bigIntPtrString(c.RootHashHinkalIndex),
		Erc20TokenAddresses:    c.Erc20TokenAddresses,
		AmountChanges:          bigIntSliceStrings(c.AmountChanges),
		InputNullifiers:        c.InputNullifiers,
		OutCommitments:         c.OutCommitments,
		EncryptedOutputs:       c.EncryptedOutputs,
		OnChainEncryptedOutput: c.OnChainEncryptedOutput,
		StealthAddressStructure: types.StealthAddressStructureJSON{
			H0x:            c.StealthAddressStructure.H0x.String(),
			H0y:            c.StealthAddressStructure.H0y.String(),
			H1x:            c.StealthAddressStructure.H1x.String(),
			H1y:            c.StealthAddressStructure.H1y.String(),
			StealthAddress: c.StealthAddressStructure.StealthAddress.String(),
		},
		TimeStamp: c.TimeStamp,
		Relay:     c.Relay,
		ExternalActionData: types.CircomExternalActionDataJSON{
			ExternalAddress:        c.ExternalActionData.ExternalAddress,
			ExternalActionID:       c.ExternalActionData.ExternalActionID.String(),
			ExternalActionMetadata: c.ExternalActionData.ExternalActionMetadata,
		},
		HookData:          c.HookData,
		CalldataHash:      c.CalldataHash.String(),
		EmporiumMessage:   c.EmporiumMessage.String(),
		PublicSignalCount: c.PublicSignalCount,
		OnChainCreation:   c.OnChainCreation,
		SlippageValues:    bigIntSliceStrings(c.SlippageValues),
		FeeStructure: types.FeeStructureJSON{
			FeeToken:     c.FeeStructure.FeeToken,
			FlatFee:      c.FeeStructure.FlatFee.String(),
			VariableRate: c.FeeStructure.VariableRate.String(),
		},
		OriginalSender:     c.OriginalSender,
		ExtraData:          c.ExtraData,
		NewRootHash:        bigIntPtrString(c.NewRootHash),
		InsertedLeafIndex:  bigIntPtrString(c.InsertedLeafIndex),
		CreateBlockedUtxos: c.CreateBlockedUtxos,
	}
}
