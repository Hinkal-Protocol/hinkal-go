package snarkjs

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type ConstructSolanaZkProofParams struct {
	GenerateProofRemotely bool
	MerkleTree            merkletree.MerkleTree
	UserKeys              *cryptokeys.UserKeys
	MintAddresses         []string
	InputUtxos            [][]*utxo.Utxo
	OutputUtxos           [][]*utxo.Utxo
	RelayerFee            *big.Int
	VariableRate          *big.Int
	RecipientAddress      string
	SignerAddress         string
	Dimensions            types.DimDataType
	OnChainCreation       []bool
	ChainID               int
	Instructions          []solanautils.HinkalInstruction
	RemainingAccounts     []solana.AccountMeta
	SwapperAccountSalt    *big.Int
	IsSpeculativeTree     bool
}

type ConstructSolanaZkProofResult struct {
	ProofAArr                []int
	ProofBArr                []int
	ProofCArr                []int
	PublicInputsArr          [][]int
	CommitmentValidationData *types.CommitmentValidationDataType
	EncryptedOutputs         [][]byte
	OnChainEncryptedOutput   []byte
	RootHashHinkalIndex      *big.Int
}

func byte32Ints(value *big.Int) []int {
	bytes := solanautils.EncodeToByte32Array(value)
	out := make([]int, len(bytes))
	for i, b := range bytes {
		out[i] = int(b)
	}
	return out
}

func appendByte32Ints(dst []int, value *big.Int) []int {
	return append(dst, byte32Ints(value)...)
}

func parseProofValues(values []string) ([]*big.Int, error) {
	out := make([]*big.Int, len(values))
	for i, value := range values {
		parsed, err := utils.ParseBigInt(value)
		if err != nil {
			return nil, err
		}
		out[i] = parsed
	}
	return out, nil
}

func flattenProofB(values [2][2]string) []string {
	return []string{values[0][0], values[0][1], values[1][0], values[1][1]}
}

func proofByteArray(values []string) ([]int, error) {
	parsed, err := parseProofValues(values)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, 32*len(parsed))
	for _, value := range parsed {
		out = appendByte32Ints(out, value)
	}
	return out, nil
}

func publicInputsByteArrays(values []string) ([][]int, error) {
	parsed, err := parseProofValues(values)
	if err != nil {
		return nil, err
	}
	out := make([][]int, len(parsed))
	for i, value := range parsed {
		out[i] = byte32Ints(value)
	}
	return out, nil
}

func ConstructSolanaZkProof(ctx context.Context, params ConstructSolanaZkProofParams) (ConstructSolanaZkProofResult, error) {
	recipientPublicKey, err := solana.PublicKeyFromBase58(params.RecipientAddress)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	signerPublicKey, err := solana.PublicKeyFromBase58(params.SignerAddress)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	nullifyingPrivateKey, err := params.UserKeys.GetShieldedPrivateKey()
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	spendingKeyPair, err := params.UserKeys.GetSpendingKeyPair()
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, nullifyingPrivateKey)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	stealthAddressStructure, err := CalcStealthAddressStructure(extraRandomization, nullifyingPrivateKey, spendingPublicKey)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	var onChainEncryptedOutput []byte
	if slices.Contains(params.OnChainCreation, true) {
		ownEncryptionKeyPair, keyErr := cryptokeys.GetEncryptionKeyPair(nullifyingPrivateKey)
		if keyErr != nil {
			return ConstructSolanaZkProofResult{}, keyErr
		}
		onChainEncryptedOutput, err = utxo.EncryptEncryptionKeyAndStealthAddress(
			ownEncryptionKeyPair.PublicKey, stealthAddressStructure.StealthAddress,
		)
		if err != nil {
			return ConstructSolanaZkProofResult{}, err
		}
	}

	calculatedEncryptedOutputs, err := CalcEncryptedOutputs(params.OutputUtxos)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	encryptedOutputs := make([][]byte, 0, len(calculatedEncryptedOutputs))
	for tokenIndex, tokenOutputs := range calculatedEncryptedOutputs {
		if tokenIndex < len(params.OnChainCreation) && params.OnChainCreation[tokenIndex] {
			encryptedOutputs = append(encryptedOutputs, nil)
			continue
		}
		for _, output := range tokenOutputs {
			encryptedOutputs = append(encryptedOutputs, common.FromHex(output))
		}
	}

	messageSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	message, err := crypto.PoseidonBig(messageSeed)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	swapperAccountSalt := params.SwapperAccountSalt
	if swapperAccountSalt == nil {
		swapperAccountSalt = big.NewInt(0)
	}
	swapperAccountAdditionalSeed, err := crypto.PoseidonBig(swapperAccountSalt)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	mintAccountPart1 := make([]*big.Int, len(params.MintAddresses))
	mintAccountPart2 := make([]*big.Int, len(params.MintAddresses))
	for i, mintAddress := range params.MintAddresses {
		formatted, err := solanautils.FormatMintAddress(mintAddress)
		if err != nil {
			return ConstructSolanaZkProofResult{}, err
		}
		mintAccountPart1[i] = formatted.MintAccountPart1
		mintAccountPart2[i] = formatted.MintAccountPart2
	}

	relayerFee := params.RelayerFee
	if relayerFee == nil {
		relayerFee = big.NewInt(0)
	}
	variableRate := params.VariableRate
	if variableRate == nil {
		variableRate = big.NewInt(0)
	}
	calldataHash := solanautils.GetSolanaCalldataHash(
		params.Dimensions,
		recipientPublicKey,
		signerPublicKey,
		encryptedOutputs,
		onChainEncryptedOutput,
		relayerFee,
		variableRate,
		params.Instructions,
		params.RemainingAccounts,
	)

	amountChanges := CalcAmountChanges(params.InputUtxos, params.OutputUtxos, false)
	data, err := GetDataFromWorkers(ctx, params.ChainID, params.MerkleTree, params.InputUtxos, params.IsSpeculativeTree)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	outCommitments, err := BuildOutCommitments(params.OutputUtxos)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	h0Ax := stealthAddressStructure.H0x

	outTimeStamp, err := utils.ParseBigInt(params.OutputUtxos[0][0].TimeStamp)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	signedMessageHash, err := ComputeSignedMessageHashSolana(SolanaSignedMessageHashParams{
		RootHashHinkal:               data.RootHashHinkal,
		MintAccountPart1:             mintAccountPart1,
		MintAccountPart2:             mintAccountPart2,
		AmountChanges:                amountChanges,
		OutTimeStamp:                 outTimeStamp,
		InNullifiers:                 data.InNullifiers,
		OutCommitments:               outCommitments,
		CalldataHash:                 calldataHash,
		Message:                      message,
		SwapperAccountAdditionalSeed: swapperAccountAdditionalSeed,
		OutH1Ay:                      stealthAddressStructure.H1y,
		H0Ax:                         h0Ax,
		H0Ay:                         stealthAddressStructure.H0y,
	})
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	sig, err := params.UserKeys.SignEddsa(signedMessageHash)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	inAmounts, err := mapUtxoStrings(params.InputUtxos, func(u *utxo.Utxo) (string, error) { return u.Amount.String(), nil })
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	inH0Ax, err := mapUtxoStrings(params.InputUtxos, func(u *utxo.Utxo) (string, error) {
		h0, err := GetUtxoCircuitH0Coords(u)
		if err != nil {
			return "", err
		}
		return h0[0].String(), nil
	})
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	inH0Ay, err := mapUtxoStrings(params.InputUtxos, func(u *utxo.Utxo) (string, error) {
		h0, err := GetUtxoCircuitH0Coords(u)
		if err != nil {
			return "", err
		}
		return h0[1].String(), nil
	})
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	outAmounts, err := mapUtxoStrings(params.OutputUtxos, func(u *utxo.Utxo) (string, error) { return u.Amount.String(), nil })
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	outPublicKeys, err := mapUtxoStrings(params.OutputUtxos, func(u *utxo.Utxo) (string, error) { return u.GetStealthAddress() })
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	inTimeStamps, err := mapUtxoStrings(params.InputUtxos, func(u *utxo.Utxo) (string, error) { return u.TimeStamp, nil })
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	dimensionString := fmt.Sprintf("%dx%dx%d", params.Dimensions.TokenNumber, params.Dimensions.NullifierAmount, params.Dimensions.OutputAmount)
	circuitInput := map[string]any{
		"rootHashHinkal":           data.RootHashHinkal,
		"spendingPublicKey":        spendingPublicKey,
		"eddsaSignature":           []*big.Int{sig.R8[0], sig.R8[1], sig.S},
		"signedMessageHash":        signedMessageHash,
		"nullifyingPrivateKey":     nullifyingPrivateKey,
		"mintAccountPart1":         mintAccountPart1,
		"mintAccountPart2":         mintAccountPart2,
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
		"calldataHash":             calldataHash,
		"messageSeed":              messageSeed,
		"swapperAccountSalt":       swapperAccountSalt,
		"H0Ax":                     h0Ax,
		"H0Ay":                     stealthAddressStructure.H0y,
	}

	mainResult, err := GenerateMainAndCommitmentZkProof(
		ctx,
		params.ChainID,
		params.UserKeys,
		params.MintAddresses,
		params.InputUtxos,
		"mainSolanaCircuit"+dimensionString,
		circuitInput,
		params.GenerateProofRemotely,
	)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	proofAArr, err := proofByteArray(mainResult.ZkCallData.A[:])
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	proofBArr, err := proofByteArray(flattenProofB(mainResult.ZkCallData.B))
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	proofCArr, err := proofByteArray(mainResult.ZkCallData.C[:])
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}
	publicInputsArr, err := publicInputsByteArrays(mainResult.PublicSignals)
	if err != nil {
		return ConstructSolanaZkProofResult{}, err
	}

	return ConstructSolanaZkProofResult{
		ProofAArr:                proofAArr,
		ProofBArr:                proofBArr,
		ProofCArr:                proofCArr,
		PublicInputsArr:          publicInputsArr,
		CommitmentValidationData: mainResult.CommitmentValidationData,
		EncryptedOutputs:         encryptedOutputs,
		OnChainEncryptedOutput:   onChainEncryptedOutput,
		RootHashHinkalIndex:      data.RootHashHinkalIndex,
	}, nil
}
