package transactions

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// Port of hinkalSolanaTransact.ts. The TS type file has no Go counterpart because internal/api
// already imports internal/types, so these types live here to avoid an import cycle.

type SolanaSubmitMode string

const (
	SolanaSubmitModeRelayer   SolanaSubmitMode = "relayer"
	SolanaSubmitModeProofOnly SolanaSubmitMode = "proof_only"
)

var (
	errSolanaTransactNoInputUtxos       = errors.New("transactions: solana transact produced no input UTXOs")
	errSolanaTransactRelayerMode        = errors.New("transactions: solana transact expected relayer submit")
	errSolanaTransactUserKeysNeedInputs = errors.New("transactions: SolanaTransact: UserKeys requires explicit InputUtxos")
	errSolanaMissingDeployData          = errors.New("missing data")
)

func ensureSolanaDeployData(chainID int) error {
	originalDeployer, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return err
	}
	if originalDeployer == "" {
		return errSolanaMissingDeployData
	}
	return nil
}

type SolanaTransactSubmit struct {
	Mode      SolanaSubmitMode
	AdminData *types.AdminDataType
}

// Args is api.SolanaArgs for transact/transfer and api.SolanaSwapArgs for swap, matching
// api.SolanaTransactionBody.Args.
// What a locally built proof does to local state, for callers that update it optimistically before
// the indexer catches up. The Solana counterpart of the CircomData fields the EVM funnel's
// OnTxConfirm gets.
type SolanaTransactNotes struct {
	InputNullifiers  []string
	OutCommitments   []string
	EncryptedOutputs []string
}

type SolanaTransactProof struct {
	Args                     any
	CommitmentValidationData *types.CommitmentValidationDataType
	// Set only on the local-proof path; the enclave never returns the notes it picked.
	Notes *SolanaTransactNotes
}

type SolanaTransactResult struct {
	TxHash string
	Proof  *SolanaTransactProof
}

// InputUtxos and OutputUtxos are overrides; when nil they are derived the same way every flow
// derived them before. UserKeys defaults to the account's own keys; claims pass a throwaway pair.
type HinkalSolanaTransactParams struct {
	ChainID            int
	MintAddresses      []string
	AmountChanges      []*big.Int
	RelayAddress       string
	Recipient          string
	Signer             string
	FunctionName       string
	Accounts           any
	OnChainCreation    []bool
	RelayerFee         *big.Int
	VariableRate       *big.Int
	SwapperAccountSalt *big.Int
	HinkalInstructions []solanautils.HinkalInstruction
	RemainingAccounts  []solana.AccountMeta
	RecipientAddress   string
	RecipientAmounts   []*big.Int
	UseBlockedUtxos    bool
	MessageSeed        *big.Int
	RecipientAmount    string
	DisplayRecipient   string
	Speculative        *types.SpeculativeTreeParams
	InputUtxos         [][]*utxo.Utxo
	UserKeys           *cryptokeys.UserKeys
	Submit             SolanaTransactSubmit
	OnTxConfirm        func(SolanaTransactNotes) error
}

func resolveSolanaInputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	params HinkalSolanaTransactParams,
) ([][]*utxo.Utxo, error) {
	if params.InputUtxos != nil {
		return params.InputUtxos, nil
	}
	return balance.AddPaddingToUtxos(
		ctx,
		hinkal,
		params.ChainID,
		params.MintAddresses,
		params.AmountChanges,
		0,
		false,
		params.UseBlockedUtxos,
		params.OnChainCreation,
	)
}

func encryptedOutputsToInts(encryptedOutputs [][]byte) [][]int {
	ints := make([][]int, len(encryptedOutputs))
	for i, row := range encryptedOutputs {
		ints[i] = make([]int, len(row))
		for j, b := range row {
			ints[i][j] = int(b)
		}
	}
	return ints
}

func buildSolanaProofLocally(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	params HinkalSolanaTransactParams,
	userKeys *cryptokeys.UserKeys,
) (SolanaTransactProof, error) {
	inputUtxosArray, err := resolveSolanaInputUtxos(ctx, hinkal, params)
	if err != nil {
		return SolanaTransactProof{}, err
	}
	if len(inputUtxosArray) == 0 || len(inputUtxosArray[0]) == 0 {
		return SolanaTransactProof{}, errSolanaTransactNoInputUtxos
	}
	outputUtxosArray, err := pretransaction.BuildOutputUtxos(
		userKeys, inputUtxosArray, params.AmountChanges, params.RecipientAddress, params.RecipientAmounts,
	)
	if err != nil {
		return SolanaTransactProof{}, err
	}

	liveMerkleTree := hinkal.MerkleTree(params.ChainID)
	merkleTree := liveMerkleTree
	if params.Speculative != nil {
		merkleTree, err = pretransaction.SpeculativeMerkleTree(liveMerkleTree, inputUtxosArray, params.Speculative.PendingLeaves)
		if err != nil {
			return SolanaTransactProof{}, err
		}
	}

	dimensions := types.DimDataType{
		TokenNumber:     len(params.MintAddresses),
		NullifierAmount: len(inputUtxosArray[0]),
		OutputAmount:    len(outputUtxosArray[0]),
	}

	relayerFee := params.RelayerFee
	if relayerFee == nil {
		relayerFee = big.NewInt(0)
	}
	variableRate := params.VariableRate
	if variableRate == nil {
		variableRate = big.NewInt(0)
	}

	proof, err := snarkjs.ConstructSolanaZkProof(ctx, snarkjs.ConstructSolanaZkProofParams{
		GenerateProofRemotely: hinkal.GenerateProofRemotely(),
		MerkleTree:            merkleTree,
		UserKeys:              userKeys,
		MintAddresses:         params.MintAddresses,
		InputUtxos:            inputUtxosArray,
		OutputUtxos:           outputUtxosArray,
		RelayerFee:            relayerFee,
		VariableRate:          variableRate,
		RecipientAddress:      params.Recipient,
		SignerAddress:         params.RelayAddress,
		Dimensions:            dimensions,
		OnChainCreation:       params.OnChainCreation,
		ChainID:               params.ChainID,
		Instructions:          params.HinkalInstructions,
		RemainingAccounts:     params.RemainingAccounts,
		SwapperAccountSalt:    params.SwapperAccountSalt,
		IsSpeculativeTree:     merkleTree != liveMerkleTree,
	})
	if err != nil {
		return SolanaTransactProof{}, err
	}

	notes, err := buildSolanaTransactNotes(inputUtxosArray, outputUtxosArray, proof.EncryptedOutputs)
	if err != nil {
		return SolanaTransactProof{}, err
	}

	args := api.SolanaArgs{
		ProofAArr:        proof.ProofAArr,
		ProofBArr:        proof.ProofBArr,
		ProofCArr:        proof.ProofCArr,
		PublicInputsArr:  proof.PublicInputsArr,
		EncryptedOutputs: encryptedOutputsToInts(proof.EncryptedOutputs),
		RelayerFee:       relayerFee.String(),
		Dimensions:       dimensions,
		RootHashIndex:    proof.RootHashHinkalIndex.String(),
	}

	if params.FunctionName == "swap" {
		variableRateString := variableRate.String()
		args.VariableRate = &variableRateString
		return SolanaTransactProof{
			Args: api.SolanaSwapArgs{
				SolanaArgs:             args,
				OnChainEncryptedOutput: "0x" + hex.EncodeToString(proof.OnChainEncryptedOutput),
				OnChainCreation:        params.OnChainCreation,
				HinkalInstructions:     hinkalInstructionsToAPI(params.HinkalInstructions),
			},
			CommitmentValidationData: proof.CommitmentValidationData,
			Notes:                    notes,
		}, nil
	}

	return SolanaTransactProof{Args: args, CommitmentValidationData: proof.CommitmentValidationData, Notes: notes}, nil
}

func buildSolanaTransactNotes(
	inputUtxosArray [][]*utxo.Utxo,
	outputUtxosArray [][]*utxo.Utxo,
	encryptedOutputs [][]byte,
) (*SolanaTransactNotes, error) {
	var inputNullifiers []string
	for _, inputUtxos := range inputUtxosArray {
		for _, inputUtxo := range inputUtxos {
			if inputUtxo.Amount == nil || inputUtxo.Amount.Sign() == 0 {
				continue
			}
			nullifier, err := inputUtxo.GetNullifier()
			if err != nil {
				return nil, err
			}
			inputNullifiers = append(inputNullifiers, nullifier)
		}
	}

	outCommitments, err := snarkjs.BuildOutCommitments(outputUtxosArray)
	if err != nil {
		return nil, err
	}

	notes := &SolanaTransactNotes{InputNullifiers: inputNullifiers}
	for _, perToken := range outCommitments {
		notes.OutCommitments = append(notes.OutCommitments, perToken...)
	}
	for _, encryptedOutput := range encryptedOutputs {
		notes.EncryptedOutputs = append(notes.EncryptedOutputs, "0x"+hex.EncodeToString(encryptedOutput))
	}
	return notes, nil
}

func hinkalInstructionsToEnclave(instructions []solanautils.HinkalInstruction) []types.PrepareSolanaTxInstruction {
	if instructions == nil {
		return nil
	}
	out := make([]types.PrepareSolanaTxInstruction, len(instructions))
	for i, instruction := range instructions {
		accountIndexes := make([]int, len(instruction.AccountIndexes))
		for j, b := range instruction.AccountIndexes {
			accountIndexes[j] = int(b)
		}
		data := make([]int, len(instruction.Data))
		for j, b := range instruction.Data {
			data[j] = int(b)
		}
		out[i] = types.PrepareSolanaTxInstruction{
			AccountIndexes: accountIndexes,
			Data:           data,
			ProgramIndex:   instruction.ProgramIndex,
		}
	}
	return out
}

func remainingAccountsToEnclave(remainingAccounts []solana.AccountMeta) []types.PrepareSolanaTxAccountMeta {
	if remainingAccounts == nil {
		return nil
	}
	out := make([]types.PrepareSolanaTxAccountMeta, len(remainingAccounts))
	for i, account := range remainingAccounts {
		out[i] = types.PrepareSolanaTxAccountMeta{
			Pubkey:     account.PublicKey.String(),
			IsSigner:   account.IsSigner,
			IsWritable: account.IsWritable,
		}
	}
	return out
}

// The enclave picks the notes itself unless the caller pinned them, in which case it gets the
// commitments of the ones it is allowed to spend.
func prepareSolanaEnclaveJob(
	ctx context.Context,
	params HinkalSolanaTransactParams,
	userKeys *cryptokeys.UserKeys,
) (string, types.EddsaSignature, error) {
	var inputCommitments []string
	var err error
	if params.Speculative == nil {
		inputCommitments, err = collectInputCommitments(params.InputUtxos)
		if err != nil {
			return "", types.EddsaSignature{}, err
		}
	}

	var accounts json.RawMessage
	if params.Accounts != nil {
		accounts, err = json.Marshal(params.Accounts)
		if err != nil {
			return "", types.EddsaSignature{}, err
		}
	}

	prepared, err := enclave.PrepareSolanaTxEnclaveCall(ctx, userKeys, types.PrepareSolanaTxParams{
		ChainID:            params.ChainID,
		MintAddresses:      params.MintAddresses,
		AmountChanges:      params.AmountChanges,
		RelayAddress:       params.RelayAddress,
		Recipient:          params.Recipient,
		Signer:             params.Signer,
		FunctionName:       params.FunctionName,
		Accounts:           accounts,
		OnChainCreation:    params.OnChainCreation,
		RelayerFee:         params.RelayerFee,
		VariableRate:       params.VariableRate,
		SwapperAccountSalt: params.SwapperAccountSalt,
		HinkalInstructions: hinkalInstructionsToEnclave(params.HinkalInstructions),
		RemainingAccounts:  remainingAccountsToEnclave(params.RemainingAccounts),
		RecipientAddress:   params.RecipientAddress,
		RecipientAmounts:   params.RecipientAmounts,
		InputCommitments:   inputCommitments,
		UseBlockedUtxos:    params.UseBlockedUtxos,
		MessageSeed:        params.MessageSeed,
		RecipientAmount:    params.RecipientAmount,
		DisplayRecipient:   params.DisplayRecipient,
		Speculative:        params.Speculative,
	})
	if err != nil {
		return "", types.EddsaSignature{}, err
	}

	signedMessageHash, err := utils.ParseBigInt(prepared.SignedMessageHash)
	if err != nil {
		return "", types.EddsaSignature{}, err
	}
	sig, err := userKeys.SignEddsa(signedMessageHash)
	if err != nil {
		return "", types.EddsaSignature{}, err
	}
	return prepared.JobID, sig, nil
}

func submitSolanaProof(
	ctx context.Context,
	params HinkalSolanaTransactParams,
	proof SolanaTransactProof,
) (string, error) {
	if params.Submit.Mode != SolanaSubmitModeRelayer {
		return "", errSolanaTransactRelayerMode
	}
	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:                  params.ChainID,
		RelayAddress:             params.RelayAddress,
		FunctionName:             params.FunctionName,
		Args:                     proof.Args,
		Accounts:                 params.Accounts,
		CommitmentValidationData: proof.CommitmentValidationData,
		AdminData:                params.Submit.AdminData,
	})
}

// SolanaTransact is the single entry point every Solana proof flow funnels through: it resolves
// the notes, builds the proof and either submits it to the relayer or hands it back.
func SolanaTransact(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	params HinkalSolanaTransactParams,
) (SolanaTransactResult, error) {
	if params.UserKeys != nil && params.InputUtxos == nil {
		return SolanaTransactResult{}, errSolanaTransactUserKeysNeedInputs
	}

	userKeys := params.UserKeys
	if userKeys == nil {
		userKeys = hinkal.GetUserKeys()
	}

	if hinkal.GenerateProofRemotely() {
		return solanaTransactViaEnclave(ctx, params, userKeys)
	}

	proof, err := buildSolanaProofLocally(ctx, hinkal, params, userKeys)
	if err != nil {
		return SolanaTransactResult{}, err
	}
	return submitAndConfirmSolana(ctx, params, proof)
}

func solanaTransactViaEnclave(
	ctx context.Context,
	params HinkalSolanaTransactParams,
	userKeys *cryptokeys.UserKeys,
) (SolanaTransactResult, error) {
	jobID, sig, err := prepareSolanaEnclaveJob(ctx, params, userKeys)
	if err != nil {
		return SolanaTransactResult{}, err
	}

	if params.Submit.Mode == SolanaSubmitModeRelayer {
		txHash, err := enclave.FinalizeSolanaTxEnclaveCallRelay(ctx, jobID, sig, params.ChainID, params.Submit.AdminData)
		if err != nil {
			return SolanaTransactResult{}, err
		}
		return SolanaTransactResult{TxHash: txHash}, nil
	}

	final, err := enclave.FinalizeSolanaTxEnclaveCall(ctx, jobID, sig)
	if err != nil {
		return SolanaTransactResult{}, err
	}
	return submitAndConfirmSolana(ctx, params, SolanaTransactProof{
		Args:                     final.SolanaArgs,
		CommitmentValidationData: final.CommitmentValidationData,
	})
}

func submitAndConfirmSolana(
	ctx context.Context,
	params HinkalSolanaTransactParams,
	proof SolanaTransactProof,
) (SolanaTransactResult, error) {
	if params.Submit.Mode != SolanaSubmitModeRelayer {
		return SolanaTransactResult{Proof: &proof}, nil
	}

	txHash, err := submitSolanaProof(ctx, params, proof)
	if err != nil {
		return SolanaTransactResult{}, err
	}
	if params.OnTxConfirm != nil && proof.Notes != nil {
		if err := params.OnTxConfirm(*proof.Notes); err != nil {
			return SolanaTransactResult{TxHash: txHash}, err
		}
	}
	return SolanaTransactResult{TxHash: txHash}, nil
}
