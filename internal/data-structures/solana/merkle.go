package solana

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
)

var merkleAccountDiscriminator = []byte{55, 52, 102, 252, 195, 69, 204, 210}

const (
	merkleDiscriminatorLen = 8
	merkleVersionLen       = 1
	merkleEntryLen         = 32
	merkleTreeLevelsCount  = int(merkletree.MerkleLevels)

	// Merkle is {version u8, tree_levels [200][32], m_index [32], levels [16], minimum_index [32]}.
	// Roots are not stored here - they live in append-only root bucket PDAs.
	merkleMIndexOffset = merkleDiscriminatorLen + merkleVersionLen + merkleTreeLevelsCount*merkleEntryLen

	rootSize = 32

	// RootBucketCap is Solana's 10 MiB account cap divided by a 32-byte root.
	RootBucketCap = 327680
)

// MerkleMinimumIndex is 2^(levels-1): the index the tree starts counting from.
func MerkleMinimumIndex() *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), uint(merkleTreeLevelsCount-1))
}

func RootBucketIndex(relativeIndex *big.Int) *big.Int {
	return new(big.Int).Div(relativeIndex, big.NewInt(RootBucketCap))
}

func RootBucketSlot(relativeIndex *big.Int) *big.Int {
	return new(big.Int).Mod(relativeIndex, big.NewInt(RootBucketCap))
}

func ParseMerkleMIndex(data []byte) (*big.Int, error) {
	minLen := merkleMIndexOffset + merkleEntryLen
	if len(data) < minLen {
		return nil, fmt.Errorf("merkle account too short: got %d, need %d", len(data), minLen)
	}
	if !bytes.Equal(data[:merkleDiscriminatorLen], merkleAccountDiscriminator) {
		return nil, fmt.Errorf("merkle account: unexpected discriminator")
	}
	return new(big.Int).SetBytes(data[merkleMIndexOffset : merkleMIndexOffset+merkleEntryLen]), nil
}

// FetchMerkleTreeRootHash reads the newest root out of its bucket PDA, mirroring the
// on-chain read_root: an unwritten slot or missing bucket reads as zero.
func FetchMerkleTreeRootHash(ctx context.Context, client *Client, programID, originalDeployer string) (*big.Int, error) {
	merkleAccount, err := GetMerkleAccountPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}
	data, err := client.GetAccountInfo(ctx, merkleAccount)
	if err != nil {
		return nil, err
	}
	mIndex, err := ParseMerkleMIndex(data)
	if err != nil {
		return nil, err
	}

	minimumIndex := MerkleMinimumIndex()
	if mIndex.Cmp(minimumIndex) <= 0 {
		return new(big.Int), nil
	}

	relativeIndex := new(big.Int).Sub(new(big.Int).Sub(mIndex, big.NewInt(1)), minimumIndex)
	bucketIndex := RootBucketIndex(relativeIndex)
	slot := RootBucketSlot(relativeIndex)

	bucketAccount, err := GetRootBucketPublicKey(programID, merkleAccount, bucketIndex)
	if err != nil {
		return nil, err
	}
	bucketData, err := client.GetAccountInfo(ctx, bucketAccount)
	if errors.Is(err, ErrAccountNotFound) {
		return new(big.Int), nil
	}
	if err != nil {
		return nil, err
	}
	start := int(slot.Int64()) * rootSize
	if len(bucketData) < start+rootSize {
		return new(big.Int), nil
	}
	return new(big.Int).SetBytes(bucketData[start : start+rootSize]), nil
}
