package snarkjs

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// Zero-valued optional fields fall back to: Relay "" -> zeroAddress, ChainID 0 -> localhost,
// OnChainCreation nil -> all false, HookData nil -> defaultHookData.
type ConstructZkProofParams struct {
	MerkleTree             merkletree.MerkleTree
	InputUtxos             [][]*utxo.Utxo
	OutputUtxos            [][]*utxo.Utxo
	UserKeys               *cryptokeys.UserKeys
	ExternalActionID       types.ExternalActionID
	ExternalAddress        string
	ExternalActionMetadata []string
	GenerateProofRemotely  bool
	FeeStructure           types.FeeStructure
	Relay                  string
	ChainID                int
	OnChainCreation        []bool
	HookData               *types.HookDataType
	OriginalSender         string
	SubAccountPrivateKey   string
	ExtraData              string
}

type ConstructZkProofResult struct {
	ZkCallData               types.NewZkCallDataType
	CircomData               types.CircomDataType
	DimData                  types.DimDataType
	EncryptedOutputs         [][]string
	CommitmentValidationData *types.CommitmentValidationDataType
}

func ConstructZkProof(ctx context.Context, params ConstructZkProofParams) (ConstructZkProofResult, error) {
	relay := params.Relay
	if relay == "" {
		relay = constants.ZeroAddress
	}
	chainID := params.ChainID
	if chainID == 0 {
		chainID = constants.ChainIDs.Localhost
	}
	onChainCreation := params.OnChainCreation
	if onChainCreation == nil {
		onChainCreation = make([]bool, len(params.InputUtxos))
	}

	inputUtxos := params.InputUtxos
	outputUtxos := params.OutputUtxos
	userKeys := params.UserKeys

	verifierName := GetZkProofVerifierName(inputUtxos, outputUtxos)
	amountChanges := CalcAmountChanges(inputUtxos, outputUtxos, false)

	erc20TokenAddresses := make([]string, len(inputUtxos))
	for i, token := range inputUtxos {
		erc20TokenAddresses[i] = token[0].Erc20TokenAddress
	}

	encryptedOutputs, err := CalcEncryptedOutputs(outputUtxos)
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	nullifyingPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, nullifyingPrivateKey)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	stealthAddressStructure, err := CalcStealthAddressStructure(extraRandomization, nullifyingPrivateKey, spendingPublicKey)
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	ownEncryptionKeyPair, err := cryptokeys.GetEncryptionKeyPair(nullifyingPrivateKey)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	onChainEncryptedOutputBytes, err := utxo.EncryptEncryptionKeyAndStealthAddress(
		ownEncryptionKeyPair.PublicKey, stealthAddressStructure.StealthAddress,
	)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	onChainEncryptedOutput := "0x" + hex.EncodeToString(onChainEncryptedOutputBytes)

	data, err := GetDataFromWorkers(ctx, chainID, params.MerkleTree, inputUtxos)
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	outCommitments, err := BuildOutCommitments(outputUtxos)
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	messageSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	emporiumMessage, err := crypto.PoseidonBig(messageSeed)
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	outTimeStamp := big.NewInt(utils.GetCurrentTimeInSeconds())
	if len(outputUtxos) > 0 {
		outTimeStamp, err = utils.ParseBigInt(outputUtxos[0][0].TimeStamp)
		if err != nil {
			return ConstructZkProofResult{}, err
		}
	}

	inAmounts, err := mapUtxoStrings(inputUtxos, func(u *utxo.Utxo) (string, error) { return u.Amount.String(), nil })
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	inH0Ax, err := mapUtxoStrings(inputUtxos, func(u *utxo.Utxo) (string, error) {
		h0, err := GetUtxoCircuitH0Coords(u)
		if err != nil {
			return "", err
		}
		return h0[0].String(), nil
	})
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	inH0Ay, err := mapUtxoStrings(inputUtxos, func(u *utxo.Utxo) (string, error) {
		h0, err := GetUtxoCircuitH0Coords(u)
		if err != nil {
			return "", err
		}
		return h0[1].String(), nil
	})
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	inTimeStamps, err := mapUtxoStrings(inputUtxos, func(u *utxo.Utxo) (string, error) { return u.TimeStamp, nil })
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	outAmounts, err := mapUtxoStrings(outputUtxos, func(u *utxo.Utxo) (string, error) { return u.Amount.String(), nil })
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	outPublicKeys, err := mapUtxoStrings(outputUtxos, func(u *utxo.Utxo) (string, error) { return u.GetStealthAddress() })
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	h0Ax := stealthAddressStructure.H0x

	input := map[string]any{
		"rootHashHinkal":           data.RootHashHinkal,
		"spendingPublicKey":        spendingPublicKey,
		"nullifyingPrivateKey":     nullifyingPrivateKey,
		"erc20TokenAddresses":      erc20TokenAddresses,
		"amountChanges":            amountChanges,
		"inAmounts":                inAmounts,
		"inH0Ax":                   inH0Ax,
		"inH0Ay":                   inH0Ay,
		"inTimeStamps":             inTimeStamps,
		"inNullifiers":             data.InNullifiers,
		"inCommitmentSiblings":     data.InCommitmentSiblings,
		"inCommitmentSiblingSides": data.InCommitmentSiblingSides,
		"outAmounts":               outAmounts,
		"outTimeStamp":             outTimeStamp,
		"outPublicKeys":            outPublicKeys,
		"outCommitments":           outCommitments,
		"messageSeed":              messageSeed,
		"H0Ax":                     h0Ax,
		"H0Ay":                     stealthAddressStructure.H0y,
	}

	publicSignalCount := CalcPublicSignalCount(verifierName, erc20TokenAddresses, amountChanges, data.InNullifiers, outCommitments)
	amountChangesBased := CalcAmountChanges(inputUtxos, outputUtxos, true)
	slippageValues := GetSlippageValues(amountChangesBased)

	metadataOps := params.ExternalActionMetadata
	var externalActionMetadata2 string
	switch {
	case params.ExternalActionID == types.ExternalActionEmporium:
		emporiumAddress := params.ExternalAddress
		if emporiumAddress == "" {
			emporiumAddress = constants.ZeroAddress
		}
		subAccountSignerAddress := ""
		if params.SubAccountPrivateKey != "" {
			subAccountSignerAddress, err = privatewallet.SignerAddressFromPrivateKey(chainID, params.SubAccountPrivateKey)
			if err != nil {
				return ConstructZkProofResult{}, err
			}
		}
		externalActionMetadata2, err = privatewallet.EncodeEmporiumMetadata(
			chainID, emporiumAddress, params.SubAccountPrivateKey, metadataOps, emporiumMessage, subAccountSignerAddress,
		)
		if err != nil {
			return ConstructZkProofResult{}, err
		}
	case params.ExternalActionID == types.ExternalActionZero || len(metadataOps) == 0:
		externalActionMetadata2 = "0x"
	default:
		externalActionMetadata2 = metadataOps[0]
	}

	extraData := params.ExtraData
	if extraData == "" {
		extraData = "0x"
	}

	calldataHash, err := CreateCallDataHash(
		chainID, publicSignalCount, relay, params.ExternalAddress, params.ExternalActionID, emporiumMessage, externalActionMetadata2,
		encryptedOutputs, onChainEncryptedOutput, params.HookData, slippageValues, onChainCreation, params.FeeStructure, params.OriginalSender, extraData,
	)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	input["calldataHash"] = calldataHash

	signedMessageHash, err := ComputeSignedMessageHashEvm(EvmSignedMessageHashParams{
		RootHashHinkal:      data.RootHashHinkal,
		Erc20TokenAddresses: erc20TokenAddresses,
		AmountChanges:       amountChanges,
		OutTimeStamp:        outTimeStamp,
		InNullifiers:        data.InNullifiers,
		OutCommitments:      outCommitments,
		CalldataHash:        calldataHash,
		Message:             emporiumMessage,
		OutH1Ax:             stealthAddressStructure.H1x,
		OutH1Ay:             stealthAddressStructure.H1y,
		H0Ax:                h0Ax,
		H0Ay:                stealthAddressStructure.H0y,
	})
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	sig, err := userKeys.SignEddsa(signedMessageHash)
	if err != nil {
		return ConstructZkProofResult{}, err
	}
	input["eddsaSignature"] = []*big.Int{sig.R8[0], sig.R8[1], sig.S}
	input["signedMessageHash"] = signedMessageHash

	isMin0Circuit := strings.HasPrefix(verifierName, "mainEVMCircuitMin0")
	var mainInput any = input
	if isMin0Circuit {
		mainInput = map[string]any{
			"outTimeStamp": outTimeStamp,
			"calldataHash": calldataHash,
			"messageSeed":  messageSeed,
		}
	}

	mainResult, err := GenerateMainAndCommitmentZkProof(
		ctx, chainID, userKeys, erc20TokenAddresses, inputUtxos, verifierName, mainInput, params.GenerateProofRemotely,
	)
	if err != nil {
		return ConstructZkProofResult{}, err
	}

	var timeStampFallback *string
	if isMin0Circuit {
		s := outTimeStamp.String()
		timeStampFallback = &s
	}

	circomData := GenerateCircomData(
		outCommitments, data.InNullifiers, data.RootHashHinkal, data.RootHashHinkalIndex, amountChangesBased, erc20TokenAddresses,
		outputUtxos, encryptedOutputs, onChainEncryptedOutput, publicSignalCount, params.ExternalActionID, params.ExternalAddress,
		emporiumMessage, externalActionMetadata2, relay, calldataHash, stealthAddressStructure, onChainCreation, params.HookData,
		timeStampFallback, slippageValues, params.FeeStructure, params.OriginalSender, extraData,
	)

	nullifierAmount := 0
	if len(inputUtxos) > 0 {
		nullifierAmount = len(inputUtxos[0])
	}
	outputAmount := 0
	if len(outputUtxos) > 0 {
		outputAmount = len(outputUtxos[0])
	}

	return ConstructZkProofResult{
		ZkCallData: mainResult.ZkCallData,
		CircomData: circomData,
		DimData: types.DimDataType{
			TokenNumber:     len(inputUtxos),
			NullifierAmount: nullifierAmount,
			OutputAmount:    outputAmount,
		},
		EncryptedOutputs:         encryptedOutputs,
		CommitmentValidationData: mainResult.CommitmentValidationData,
	}, nil
}

func mapUtxoStrings(utxos [][]*utxo.Utxo, fn func(*utxo.Utxo) (string, error)) ([][]string, error) {
	out := make([][]string, len(utxos))
	for i, token := range utxos {
		out[i] = make([]string, len(token))
		for j, u := range token {
			v, err := fn(u)
			if err != nil {
				return nil, err
			}
			out[i][j] = v
		}
	}
	return out, nil
}
