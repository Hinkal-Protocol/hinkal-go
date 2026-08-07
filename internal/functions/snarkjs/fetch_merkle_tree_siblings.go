package snarkjs

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type MerkleTreeSiblingsResponse struct {
	InCommitmentSiblings     [][][]string `json:"inCommitmentSiblings"`
	InCommitmentSiblingSides [][][]string `json:"inCommitmentSiblingSides"`
	RootHashHinkal           string       `json:"rootHashHinkal"`
	RootHashHinkalIndex      string       `json:"rootHashHinkalIndex"`
}

func FetchMerkleTreeSiblings(ctx context.Context, chainID int, inputUtxos [][]*utxo.Utxo) (*MerkleTreeSiblingsResponse, error) {
	serialized := make([][]types.UtxoParams, len(inputUtxos))
	for i, token := range inputUtxos {
		serialized[i] = make([]types.UtxoParams, len(token))
		for j, u := range token {
			serialized[i][j] = u.GetConstructableParams()
		}
	}

	body := map[string]any{
		"inputUtxosSerialized": serialized,
		"chainId":              chainID,
	}

	url := constants.GetSnapshotServerURL() + constants.SnapshotServerConfig.MerkleTreeSiblings
	var resp MerkleTreeSiblingsResponse
	if err := api.Post(ctx, url, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
