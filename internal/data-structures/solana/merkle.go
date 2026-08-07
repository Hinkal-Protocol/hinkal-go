package solana

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
)

var merkleAccountDiscriminator = []byte{55, 52, 102, 252, 195, 69, 204, 210}

const (
	merkleDiscriminatorLen = 8
	merkleVersionLen       = 1
	merkleEntryLen         = 32
	merkleTreeLevelsCount  = 200
	merkleRootsCount       = 100

	merkleRootsOffset     = merkleDiscriminatorLen + merkleVersionLen + merkleTreeLevelsCount*merkleEntryLen
	merkleRootIndexOffset = merkleRootsOffset + merkleRootsCount*merkleEntryLen + merkleEntryLen
)

func FetchMerkleTreeRootHash(ctx context.Context, client *Client, programID, originalDeployer string) (*big.Int, error) {
	merkleAccount, err := GetMerkleAccountPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}
	data, err := client.GetAccountInfo(ctx, merkleAccount)
	if err != nil {
		return nil, err
	}
	return ParseMerkleRootHash(data)
}

func ParseMerkleRootHash(data []byte) (*big.Int, error) {
	minLen := merkleRootIndexOffset + merkleEntryLen
	if len(data) < minLen {
		return nil, fmt.Errorf("merkle account too short: got %d, need %d", len(data), minLen)
	}
	if !bytes.Equal(data[:merkleDiscriminatorLen], merkleAccountDiscriminator) {
		return nil, fmt.Errorf("merkle account: unexpected discriminator")
	}

	rootIndex := new(big.Int).SetBytes(data[merkleRootIndexOffset : merkleRootIndexOffset+merkleEntryLen])
	if rootIndex.Cmp(big.NewInt(merkleRootsCount)) > 0 {
		return nil, fmt.Errorf("merkle account: root index %s out of range", rootIndex)
	}

	idx := rootIndex.Int64() - 1
	if idx < 0 {
		idx += merkleRootsCount
	}

	start := merkleRootsOffset + int(idx)*merkleEntryLen
	rootBytes := data[start : start+merkleEntryLen]
	return new(big.Int).SetBytes(rootBytes), nil
}
