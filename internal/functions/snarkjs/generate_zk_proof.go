package snarkjs

import (
	"context"
	"errors"
)

func GenerateZkProof(
	ctx context.Context,
	chainID int,
	verifierNames []string,
	inputs []any,
	remotely bool,
) ([]ZkProofResult, error) {
	if remotely {
		results, err := GenerateZkProofEnclave(ctx, chainID, verifierNames, inputs)
		if err == nil {
			return results, nil
		}
		selfResults, selfErr := generateZkProofSelfBatch(ctx, chainID, verifierNames, inputs)
		if selfErr != nil {
			return nil, errors.Join(err, selfErr)
		}
		return selfResults, nil
	}
	return generateZkProofSelfBatch(ctx, chainID, verifierNames, inputs)
}

func generateZkProofSelfBatch(
	ctx context.Context,
	chainID int,
	verifierNames []string,
	inputs []any,
) ([]ZkProofResult, error) {
	results := make([]ZkProofResult, len(verifierNames))
	for i, name := range verifierNames {
		r, err := GenerateZkProofSelf(ctx, chainID, name, inputs[i])
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}
