package pretransaction

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errPartialPendingLeaves = errors.New("only some pending leaves are already in the Merkle tree")

func ToSpeculativeUtxo(u *utxo.Utxo) (types.SpeculativeUtxo, error) {
	stealthAddress, err := u.GetStealthAddress()
	if err != nil {
		return types.SpeculativeUtxo{}, err
	}
	stealthAddressBig, err := utils.ParseBigInt(stealthAddress)
	if err != nil {
		return types.SpeculativeUtxo{}, err
	}
	h0 := [2]string{"0", "0"}
	if u.H0 != nil {
		for i, coordinate := range u.H0 {
			if coordinate != nil {
				h0[i] = coordinate.String()
			}
		}
	}
	return types.SpeculativeUtxo{
		Amount:            u.Amount.String(),
		Erc20TokenAddress: u.Erc20TokenAddress,
		MintAddress:       u.MintAddress,
		TimeStamp:         u.TimeStamp,
		StealthAddress:    stealthAddressBig.String(),
		IsBlocked:         u.IsBlocked,
		H0:                h0,
	}, nil
}

func ToSpeculativeUtxos(inputUtxosArray [][]*utxo.Utxo) ([][]types.SpeculativeUtxo, error) {
	out := make([][]types.SpeculativeUtxo, len(inputUtxosArray))
	for i, notes := range inputUtxosArray {
		out[i] = make([]types.SpeculativeUtxo, len(notes))
		for j, note := range notes {
			speculative, err := ToSpeculativeUtxo(note)
			if err != nil {
				return nil, err
			}
			out[i][j] = speculative
		}
	}
	return out, nil
}

// SpeculativeMerkleTree returns the tree to prove against while the deposit that creates
// pendingCommitments is still in flight: the live tree with those leaves appended, or the live
// tree itself once the indexer caught up.
func SpeculativeMerkleTree(
	merkleTree merkletree.MerkleTree,
	inputUtxosArray [][]*utxo.Utxo,
	pendingCommitments []string,
) (merkletree.MerkleTree, error) {
	if merkleTree == nil {
		return nil, errors.New("pre-transaction: no merkle tree to build a speculative tree from")
	}
	pendingLeaves := make([]*big.Int, len(pendingCommitments))
	presentLeafCount := 0
	for i, commitment := range pendingCommitments {
		leaf, err := utils.ParseBigInt(commitment)
		if err != nil {
			return nil, err
		}
		pendingLeaves[i] = leaf
		if merkleTree.Contains(leaf) {
			presentLeafCount++
		}
	}
	if presentLeafCount > 0 && presentLeafCount != len(pendingLeaves) {
		return nil, errPartialPendingLeaves
	}

	speculativeTree := merkleTree
	if presentLeafCount != len(pendingLeaves) {
		speculativeTree = merkleTree.Clone()
		for _, leaf := range pendingLeaves {
			speculativeTree.Insert(leaf, speculativeTree.GetIndex())
		}
	}

	for _, notes := range inputUtxosArray {
		for _, note := range notes {
			if note.Amount == nil || note.Amount.Sign() <= 0 {
				continue
			}
			commitment, err := note.GetCommitment()
			if err != nil {
				return nil, err
			}
			commitmentBig, err := utils.ParseBigInt(commitment)
			if err != nil {
				return nil, err
			}
			if !speculativeTree.Contains(commitmentBig) {
				return nil, fmt.Errorf("input commitment %s is not in the speculative Merkle tree", commitment)
			}
		}
	}

	return speculativeTree, nil
}
