package hinkal

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type commitmentsSnapshot interface {
	EncryptedOutputs() []*types.EncryptedOutputWithSign
	MerkleTree() merkletree.MerkleTree
	RetrieveEventsFromLatestBlock(ctx context.Context) error
	IntervalClear()
}

type nullifierSnapshot interface {
	Nullifiers() map[string]struct{}
	IntervalClear()
}
