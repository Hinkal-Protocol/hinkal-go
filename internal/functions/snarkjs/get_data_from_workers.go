package snarkjs

import (
	"context"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type MerkleDataFromWorkers struct {
	InCommitmentSiblings     [][][]string
	InCommitmentSiblingSides [][][]string
	RootHashHinkal           *big.Int
	RootHashHinkalIndex      *big.Int
	InNullifiers             [][]string
}

type merkleTreeSiblingsAndRootHashes struct {
	inCommitmentSiblings     [][][]string
	inCommitmentSiblingSides [][][]string
	rootHashHinkal           *big.Int
	rootHashHinkalIndex      *big.Int
}

// zero-amount UTXOs nullify to "0" (worker handleBuildInNullifiers semantics, which zeroes by
// amount, not by onChainCreation).
func BuildInNullifiersByAmount(inputUtxos [][]*utxo.Utxo) ([][]string, error) {
	out := make([][]string, len(inputUtxos))
	for i, token := range inputUtxos {
		out[i] = make([]string, len(token))
		for j, u := range token {
			if u.Amount.Sign() == 0 {
				out[i][j] = "0"
				continue
			}
			n, err := u.GetNullifier()
			if err != nil {
				return nil, err
			}
			out[i][j] = n
		}
	}
	return out, nil
}

func handleLocalMerkleTrees(merkleTree merkletree.MerkleTree, inputUtxos [][]*utxo.Utxo) (merkleTreeSiblingsAndRootHashes, error) {
	if merkleTree == nil {
		return merkleTreeSiblingsAndRootHashes{}, errors.New("root hash not available from hinkal merkle tree")
	}
	rootHash, err := merkleTree.GetRootHash()
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	rootHashIndex := new(big.Int).Sub(merkleTree.GetIndex(), big.NewInt(1))
	siblings, sides, err := CalcCommitmentsSiblingAndSides(inputUtxos, merkleTree)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	return merkleTreeSiblingsAndRootHashes{
		inCommitmentSiblings:     siblings,
		inCommitmentSiblingSides: sides,
		rootHashHinkal:           rootHash,
		rootHashHinkalIndex:      rootHashIndex,
	}, nil
}

func handleRemoteMerkleTrees(ctx context.Context, chainID int, inputUtxos [][]*utxo.Utxo) (merkleTreeSiblingsAndRootHashes, error) {
	resp, err := FetchMerkleTreeSiblings(ctx, chainID, inputUtxos)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	rootHash, err := utils.ParseBigInt(resp.RootHashHinkal)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	rootHashIndex, err := utils.ParseBigInt(resp.RootHashHinkalIndex)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	return merkleTreeSiblingsAndRootHashes{
		inCommitmentSiblings:     resp.InCommitmentSiblings,
		inCommitmentSiblingSides: resp.InCommitmentSiblingSides,
		rootHashHinkal:           rootHash,
		rootHashHinkalIndex:      rootHashIndex,
	}, nil
}

func areLocalTreesUpToDate(ctx context.Context, chainID int, merkleTree merkletree.MerkleTree) bool {
	if merkleTree == nil {
		return false
	}
	localRoot, err := merkleTree.GetRootHash()
	if err != nil {
		return false
	}
	onChainRoot, err := FetchOnChainRootHashes(ctx, chainID)
	if err != nil {
		return false
	}
	return localRoot.Cmp(onChainRoot) == 0
}

func GetMerkleTreeSiblingsAndRootHashes(ctx context.Context, chainID int, merkleTree merkletree.MerkleTree, inputUtxos [][]*utxo.Utxo, isSpeculativeTree bool) (merkleTreeSiblingsAndRootHashes, error) {
	if isSpeculativeTree {
		return handleLocalMerkleTrees(merkleTree, inputUtxos)
	}
	if areLocalTreesUpToDate(ctx, chainID, merkleTree) || constants.IsLocalNetwork(chainID) {
		return handleLocalMerkleTrees(merkleTree, inputUtxos)
	}
	return handleRemoteMerkleTrees(ctx, chainID, inputUtxos)
}

func GetDataFromWorkers(ctx context.Context, chainID int, merkleTree merkletree.MerkleTree, inputUtxos [][]*utxo.Utxo, isSpeculativeTree bool) (MerkleDataFromWorkers, error) {
	if HasOnlyZeroAmounts(inputUtxos) {
		zeroData := BuildZeroInputMerkleDataFromSerialized(inputUtxos)
		var rootHash, rootHashIndex *big.Int
		if constants.IsLocalNetwork(chainID) && merkleTree != nil {
			var err error
			rootHash, err = merkleTree.GetRootHash()
			if err != nil {
				return MerkleDataFromWorkers{}, err
			}
			rootHashIndex = new(big.Int).Sub(merkleTree.GetIndex(), big.NewInt(1))
		} else {
			var err error
			rootHash, rootHashIndex, err = FetchOnChainRootHashAndIndex(ctx, chainID)
			if err != nil {
				return MerkleDataFromWorkers{}, err
			}
		}
		return MerkleDataFromWorkers{
			InCommitmentSiblings:     zeroData.InCommitmentSiblings,
			InCommitmentSiblingSides: zeroData.InCommitmentSiblingSides,
			RootHashHinkal:           rootHash,
			RootHashHinkalIndex:      rootHashIndex,
			InNullifiers:             zeroData.InNullifiers,
		}, nil
	}

	siblings, err := GetMerkleTreeSiblingsAndRootHashes(ctx, chainID, merkleTree, inputUtxos, isSpeculativeTree)
	if err != nil {
		return MerkleDataFromWorkers{}, err
	}

	nullifiers, err := BuildInNullifiersByAmount(inputUtxos)
	if err != nil {
		return MerkleDataFromWorkers{}, err
	}

	return MerkleDataFromWorkers{
		InCommitmentSiblings:     siblings.inCommitmentSiblings,
		InCommitmentSiblingSides: siblings.inCommitmentSiblingSides,
		RootHashHinkal:           siblings.rootHashHinkal,
		RootHashHinkalIndex:      siblings.rootHashHinkalIndex,
		InNullifiers:             nullifiers,
	}, nil
}
