package snapshot

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type CommitmentsPayload struct {
	MerkleTree       merkletree.MerkleTree
	EncryptedOutputs []*types.EncryptedOutputWithSign
	ByCommitment     map[string]string
}

type CommitmentsSnapshotService struct {
	Svc *eventservice.SnapshotService[
		*types.CommitmentEvent,
		*CommitmentsPayload,
		*types.CommitmentsSerializedSnapshot,
	]
	merkleTree   merkletree.MerkleTree
	encOutputs   []*types.EncryptedOutputWithSign
	byCommitment map[string]string
	hashFn       merkletree.HashFunc
	fetchFn      func(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error)
	persistFn    func(ctx context.Context, s *types.CommitmentsSerializedSnapshot) error
}

func NewCommitmentsSnapshotService(
	emitter eventservice.EventEmitter,
	hashFn merkletree.HashFunc,
	fetchFn func(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error),
	persistFn func(ctx context.Context, s *types.CommitmentsSerializedSnapshot) error,
) *CommitmentsSnapshotService {
	svc := &CommitmentsSnapshotService{
		byCommitment: make(map[string]string),
		hashFn:       hashFn,
		fetchFn:      fetchFn,
		persistFn:    persistFn,
	}
	svc.Svc = eventservice.NewSnapshotService(emitter, "NewCommitment", svc)
	return svc
}

func NewClientCommitmentsSnapshotService(
	emitter eventservice.EventEmitter,
	hashFn merkletree.HashFunc,
	fetcher *SnapshotFetcherService,
) *CommitmentsSnapshotService {
	return NewCommitmentsSnapshotService(
		emitter, hashFn,
		func(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error) {
			return fetcher.GetCommitments(ctx)
		},
		func(_ context.Context, _ *types.CommitmentsSerializedSnapshot) error { return nil },
	)
}

func (s *CommitmentsSnapshotService) MerkleTree() merkletree.MerkleTree { return s.merkleTree }
func (s *CommitmentsSnapshotService) EncryptedOutputs() []*types.EncryptedOutputWithSign {
	return s.encOutputs
}
func (s *CommitmentsSnapshotService) RetrieveEventsFromLatestBlock(ctx context.Context) error {
	return s.Svc.RetrieveEventsFromLatestBlock(ctx)
}

func (s *CommitmentsSnapshotService) IntervalClear() { s.Svc.IntervalClear() }

func (s *CommitmentsSnapshotService) FetchSnapshot(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error) {
	return s.fetchFn(ctx)
}

func (s *CommitmentsSnapshotService) PersistSnapshot(ctx context.Context, snap *types.CommitmentsSerializedSnapshot) error {
	return s.persistFn(ctx, snap)
}

func (s *CommitmentsSnapshotService) GetSnapshotPayload() *CommitmentsPayload {
	return &CommitmentsPayload{
		MerkleTree:       s.merkleTree,
		EncryptedOutputs: s.encOutputs,
		ByCommitment:     s.byCommitment,
	}
}

func (s *CommitmentsSnapshotService) PopulateSnapshot(snap eventservice.Snapshot[*CommitmentsPayload]) {
	s.merkleTree = snap.Payload.MerkleTree
	s.encOutputs = snap.Payload.EncryptedOutputs
	s.byCommitment = snap.Payload.ByCommitment
}

func (s *CommitmentsSnapshotService) SerializeSnapshot(
	snap eventservice.Snapshot[*CommitmentsPayload],
) (*types.CommitmentsSerializedSnapshot, error) {
	block := snap.LatestBlockNumber
	merkleJSON := snap.Payload.MerkleTree.ToJSON()
	serializedOutputs := make([]types.SerializedEncryptedOutputWithSign, len(snap.Payload.EncryptedOutputs))
	for i, out := range snap.Payload.EncryptedOutputs {
		serializedOutputs[i] = types.SerializedEncryptedOutputWithSign{
			Value:      out.Value,
			IsPositive: strconv.FormatBool(out.IsPositive),
			IsBlocked:  out.IsBlocked,
		}
	}
	commitments := make([]string, 0, len(snap.Payload.ByCommitment))
	for commitment := range snap.Payload.ByCommitment {
		commitments = append(commitments, commitment)
	}
	sort.Slice(commitments, func(i, j int) bool {
		a, _ := new(big.Int).SetString(commitments[i], 10)
		b, _ := new(big.Int).SetString(commitments[j], 10)
		return a.Cmp(b) < 0
	})
	byCommitment := make([]types.EncryptedOutputWithCommitment, 0, len(commitments))
	for _, commitment := range commitments {
		byCommitment = append(byCommitment, types.EncryptedOutputWithCommitment{
			Commitment:      commitment,
			EncryptedOutput: snap.Payload.ByCommitment[commitment],
		})
	}
	return &types.CommitmentsSerializedSnapshot{
		LatestBlockNumber:            &block,
		MerkleTree:                   &merkleJSON,
		EncryptedOutputs:             serializedOutputs,
		EncryptedOutputsByCommitment: byCommitment,
	}, nil
}

func (s *CommitmentsSnapshotService) DeserializeSnapshot(
	serialized *types.CommitmentsSerializedSnapshot,
) (eventservice.Snapshot[*CommitmentsPayload], error) {
	empty := eventservice.Snapshot[*CommitmentsPayload]{}
	var tree merkletree.MerkleTree
	if serialized.LatestBlockNumber != nil && *serialized.LatestBlockNumber > 0 && serialized.MerkleTree != nil {
		var err error
		tree, err = merkletree.FromJSON(*serialized.MerkleTree, s.hashFn, new(big.Int))
		if err != nil {
			return empty, fmt.Errorf("deserialize merkle tree: %w", err)
		}
	} else {
		tree = merkletree.New(s.hashFn, new(big.Int), 0)
	}
	encOutputs := make([]*types.EncryptedOutputWithSign, 0, len(serialized.EncryptedOutputs))
	for _, out := range serialized.EncryptedOutputs {
		isPositive, parseErr := strconv.ParseBool(out.IsPositive)
		if parseErr != nil {
			return empty, fmt.Errorf("parse IsPositive %q: %w", out.IsPositive, parseErr)
		}
		encOutputs = append(encOutputs, &types.EncryptedOutputWithSign{
			Value:      out.Value,
			IsPositive: isPositive,
			IsBlocked:  out.IsBlocked,
		})
	}
	byCommitment := make(map[string]string, len(serialized.EncryptedOutputsByCommitment))
	for _, entry := range serialized.EncryptedOutputsByCommitment {
		byCommitment[entry.Commitment] = entry.EncryptedOutput
	}
	latestBlock := uint64(0)
	if serialized.LatestBlockNumber != nil {
		latestBlock = *serialized.LatestBlockNumber
	}
	return eventservice.Snapshot[*CommitmentsPayload]{
		LatestBlockNumber: latestBlock,
		Payload: &CommitmentsPayload{
			MerkleTree:       tree,
			EncryptedOutputs: encOutputs,
			ByCommitment:     byCommitment,
		},
	}, nil
}

func (s *CommitmentsSnapshotService) MapEvent(ev *blockchainevent.BlockchainEvent) (*types.CommitmentEvent, error) {
	commitmentStr, err := ev.GetArg("commitment")
	if err != nil {
		return nil, err
	}
	indexStr, err := ev.GetArg("index")
	if err != nil {
		return nil, err
	}
	encryptedOutput, err := ev.GetArg("encryptedOutput")
	if err != nil {
		return nil, err
	}
	commitment, err := utils.ParseBigInt(commitmentStr)
	if err != nil {
		return nil, fmt.Errorf("parse commitment: %w", err)
	}
	index, err := utils.ParseBigInt(indexStr)
	if err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	return &types.CommitmentEvent{Commitment: commitment, Index: index, EncryptedOutput: encryptedOutput}, nil
}

func (s *CommitmentsSnapshotService) AcceptEvent(
	event *types.CommitmentEvent, _ uint64, _ string, isBlocked bool,
) bool {
	nodeIndex := new(big.Int).Abs(event.Index)
	existing, exists := s.merkleTree.GetValue(nodeIndex)
	if exists && existing.Cmp(event.Commitment) == 0 {
		return false
	}
	s.merkleTree.Insert(event.Commitment, nodeIndex)
	if exists && existing != nil {
		if oldEncOut, ok := s.byCommitment[existing.String()]; ok {
			for i, out := range s.encOutputs {
				if out.Value == oldEncOut {
					s.encOutputs = append(s.encOutputs[:i], s.encOutputs[i+1:]...)
					break
				}
			}
			delete(s.byCommitment, existing.String())
		}
	}
	s.byCommitment[event.Commitment.String()] = event.EncryptedOutput
	for _, out := range s.encOutputs {
		if out.Value == event.EncryptedOutput {
			return true
		}
	}
	s.encOutputs = append(s.encOutputs, &types.EncryptedOutputWithSign{
		Value:      event.EncryptedOutput,
		IsPositive: event.Index.Sign() >= 0,
		IsBlocked:  isBlocked,
	})
	return true
}
