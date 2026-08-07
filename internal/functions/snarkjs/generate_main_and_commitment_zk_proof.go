package snarkjs

import (
	"context"

	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type MainAndCommitmentResult struct {
	ZkCallData               types.NewZkCallDataType
	PublicSignals            []string
	CommitmentValidationData *types.CommitmentValidationDataType
}

func GenerateMainAndCommitmentZkProof(
	ctx context.Context,
	chainID int,
	userKeys *cryptokeys.UserKeys,
	tokenAddresses []string,
	inputUtxos [][]*utxo.Utxo,
	mainVerifierName string,
	mainCircuitInput any,
	generateProofRemotely bool,
) (MainAndCommitmentResult, error) {
	build, err := BuildCommitmentValidationData(chainID, userKeys, tokenAddresses, inputUtxos)
	if err != nil {
		return MainAndCommitmentResult{}, err
	}

	verifierNames := []string{mainVerifierName}
	inputs := []any{mainCircuitInput}
	if build != nil {
		verifierNames = append(verifierNames, build.VerifierName)
		inputs = append(inputs, build.CommitmentInput)
	}

	proofResponses, err := GenerateZkProof(ctx, chainID, verifierNames, inputs, generateProofRemotely)
	if err != nil {
		return MainAndCommitmentResult{}, err
	}

	var commitmentProof *ZkProofResult
	if build != nil && len(proofResponses) > 1 {
		commitmentProof = &proofResponses[1]
	}
	commitmentData := BuildCommitmentValidationDataFromProof(build, commitmentProof)

	return MainAndCommitmentResult{
		ZkCallData:               proofResponses[0].ZkCallData,
		PublicSignals:            proofResponses[0].PublicSignals,
		CommitmentValidationData: commitmentData,
	}, nil
}
