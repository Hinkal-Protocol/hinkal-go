package snarkjs

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GenerateCircomData(
	outCommitments [][]string,
	inputNullifiers [][]string,
	rootHashHinkal *big.Int,
	rootHashHinkalIndex *big.Int,
	amountChanges []*big.Int,
	erc20TokenAddresses []string,
	outputUtxos [][]*utxo.Utxo,
	encryptedOutputs [][]string,
	onChainEncryptedOutput string,
	publicSignalCount int,
	externalActionID types.ExternalActionID,
	externalAddress string,
	emporiumMessage *big.Int,
	externalActionMetadata string,
	relay string,
	calldataHash *big.Int,
	stealthAddressStructure types.StealthAddressStructure,
	onChainCreation []bool,
	hookData *types.HookDataType,
	timeStampFallback *string,
	slippageValues []*big.Int,
	feeStructure types.FeeStructure,
	originalSender string,
	extraData string,
) types.CircomDataType {
	timeStamp := ""
	if len(outputUtxos) > 0 {
		timeStamp = outputUtxos[0][0].TimeStamp
	} else if timeStampFallback != nil {
		timeStamp = *timeStampFallback
	}

	hd := types.DefaultHookData()
	if hookData != nil {
		hd = *hookData
	}

	externalAddressOrZero := externalAddress
	if externalAddressOrZero == "" {
		externalAddressOrZero = constants.ZeroAddress
	}

	sender := originalSender
	if sender == "" {
		sender = GetOriginalSender(externalAddressOrZero, relay)
	}

	return types.CircomDataType{
		RootHashHinkal:          rootHashHinkal,
		RootHashHinkalIndex:     rootHashHinkalIndex,
		Erc20TokenAddresses:     erc20TokenAddresses,
		AmountChanges:           amountChanges,
		InputNullifiers:         inputNullifiers,
		OutCommitments:          outCommitments,
		EncryptedOutputs:        encryptedOutputs,
		OnChainEncryptedOutput:  onChainEncryptedOutput,
		TimeStamp:               timeStamp,
		StealthAddressStructure: stealthAddressStructure,
		Relay:                   relay,
		ExternalActionData: types.CircomExternalActionData{
			ExternalAddress:        externalAddressOrZero,
			ExternalActionID:       GetExternalActionIDHash(externalActionID),
			ExternalActionMetadata: externalActionMetadata,
		},
		HookData:          hd,
		PublicSignalCount: publicSignalCount,
		CalldataHash:      calldataHash,
		EmporiumMessage:   emporiumMessage,
		OnChainCreation:   onChainCreation,
		SlippageValues:    slippageValues,
		FeeStructure:      feeStructure,
		OriginalSender:    sender,
		ExtraData:         extraData,
	}
}
