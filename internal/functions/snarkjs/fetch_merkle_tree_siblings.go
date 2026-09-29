package snarkjs

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type MerkleTreeSiblingsResponse struct {
	InCommitmentSiblings     [][][]string `json:"inCommitmentSiblings"`
	InCommitmentSiblingSides [][][]string `json:"inCommitmentSiblingSides"`
	RootHashHinkal           string       `json:"rootHashHinkal"`
	RootHashHinkalIndex      string       `json:"rootHashHinkalIndex"`
}

func FetchMerkleTreeSiblings(ctx context.Context, chainID int, inputUtxos [][]*utxo.Utxo) (*MerkleTreeSiblingsResponse, error) {
	inputCommitments := make([][]string, len(inputUtxos))
	for i, token := range inputUtxos {
		inputCommitments[i] = make([]string, len(token))
		for j, u := range token {
			if u.Amount == nil || u.Amount.Sign() == 0 {
				inputCommitments[i][j] = "0"
				continue
			}
			commitment, err := u.GetCommitment()
			if err != nil {
				return nil, err
			}
			commitmentBig, err := utils.ParseBigInt(commitment)
			if err != nil {
				return nil, err
			}
			inputCommitments[i][j] = commitmentBig.String()
		}
	}

	body := map[string]any{
		"inputCommitments": inputCommitments,
		"chainId":          chainID,
	}

	url := constants.GetSnapshotServerURL() + constants.SnapshotServerConfig.MerkleTreeSiblings
	var resp MerkleTreeSiblingsResponse
	if err := api.Post(ctx, url, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
