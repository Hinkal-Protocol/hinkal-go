package snapshot

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SolanaCommitmentsSnapshotService struct {
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

func hasOnChainData(rows [][]byte) bool {
	for _, row := range rows {
		for _, b := range row {
			if b != 0 {
				return true
			}
		}
	}
	return false
}

func NewSolanaCommitmentsSnapshotService(
	emitter eventservice.EventEmitter,
	hashFn merkletree.HashFunc,
	fetchFn func(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error),
	persistFn func(ctx context.Context, s *types.CommitmentsSerializedSnapshot) error,
) *SolanaCommitmentsSnapshotService {
	svc := &SolanaCommitmentsSnapshotService{
		byCommitment: make(map[string]string),
		hashFn:       hashFn,
		fetchFn:      fetchFn,
		persistFn:    persistFn,
	}
	svc.Svc = eventservice.NewSnapshotService(emitter, "NewCommitment", svc)
	return svc
}

func NewClientSolanaCommitmentsSnapshotService(
	emitter eventservice.EventEmitter,
	hashFn merkletree.HashFunc,
	fetcher *SnapshotFetcherService,
) *SolanaCommitmentsSnapshotService {
	return NewSolanaCommitmentsSnapshotService(
		emitter, hashFn,
		func(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error) {
			return fetcher.GetCommitments(ctx)
		},
		func(_ context.Context, _ *types.CommitmentsSerializedSnapshot) error { return nil },
	)
}

func (s *SolanaCommitmentsSnapshotService) MerkleTree() merkletree.MerkleTree { return s.merkleTree }
func (s *SolanaCommitmentsSnapshotService) EncryptedOutputs() []*types.EncryptedOutputWithSign {
	return s.encOutputs
}
func (s *SolanaCommitmentsSnapshotService) RetrieveEventsFromLatestBlock(ctx context.Context) error {
	return s.Svc.RetrieveEventsFromLatestBlock(ctx)
}

func (s *SolanaCommitmentsSnapshotService) IntervalClear() { s.Svc.IntervalClear() }

func (s *SolanaCommitmentsSnapshotService) FetchSnapshot(ctx context.Context) (*types.CommitmentsSerializedSnapshot, error) {
	return s.fetchFn(ctx)
}

func (s *SolanaCommitmentsSnapshotService) PersistSnapshot(ctx context.Context, snap *types.CommitmentsSerializedSnapshot) error {
	return s.persistFn(ctx, snap)
}

func (s *SolanaCommitmentsSnapshotService) GetSnapshotPayload() *CommitmentsPayload {
	return &CommitmentsPayload{
		MerkleTree:       s.merkleTree,
		EncryptedOutputs: s.encOutputs,
		ByCommitment:     s.byCommitment,
	}
}

func (s *SolanaCommitmentsSnapshotService) PopulateSnapshot(snap eventservice.Snapshot[*CommitmentsPayload]) {
	s.merkleTree = snap.Payload.MerkleTree
	s.encOutputs = snap.Payload.EncryptedOutputs
	s.byCommitment = snap.Payload.ByCommitment
}

func (s *SolanaCommitmentsSnapshotService) SerializeSnapshot(
	snap eventservice.Snapshot[*CommitmentsPayload],
) (*types.CommitmentsSerializedSnapshot, error) {
	block := snap.LatestBlockNumber
	merkleJSON := snap.Payload.MerkleTree.ToJSON()
	serializedOutputs := make([]types.SerializedEncryptedOutputWithSign, len(snap.Payload.EncryptedOutputs))
	for i, out := range snap.Payload.EncryptedOutputs {
		serializedOutputs[i] = types.SerializedEncryptedOutputWithSign{
			Value:         out.Value,
			IsPositive:    strconv.FormatBool(out.IsPositive),
			IsBlocked:     out.IsBlocked,
			EncryptedBlob: out.EncryptedBlob,
		}
	}
	byCommitment := make([]types.EncryptedOutputWithCommitment, 0, len(snap.Payload.ByCommitment))
	for commitment, encOut := range snap.Payload.ByCommitment {
		byCommitment = append(byCommitment, types.EncryptedOutputWithCommitment{
			Commitment:      commitment,
			EncryptedOutput: encOut,
		})
	}
	return &types.CommitmentsSerializedSnapshot{
		LatestBlockNumber:            &block,
		MerkleTree:                   &merkleJSON,
		EncryptedOutputs:             serializedOutputs,
		EncryptedOutputsByCommitment: byCommitment,
	}, nil
}

func (s *SolanaCommitmentsSnapshotService) DeserializeSnapshot(
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
			Value:         out.Value,
			IsPositive:    isPositive,
			IsBlocked:     out.IsBlocked,
			EncryptedBlob: out.EncryptedBlob,
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

func (s *SolanaCommitmentsSnapshotService) MapEvent(ev *blockchainevent.BlockchainEvent) (*types.CommitmentEvent, error) {
	commitmentArg, err := ev.GetArg("commitment")
	if err != nil {
		return nil, err
	}
	indexArg, err := ev.GetArg("index")
	if err != nil {
		return nil, err
	}
	encOutArg, err := ev.GetArg("encrypted_output")
	if err != nil {
		return nil, err
	}
	commitment, err := utils.AdvancedToBigInt(commitmentArg)
	if err != nil {
		return nil, fmt.Errorf("parse commitment: %w", err)
	}
	index, err := utils.AdvancedToBigInt(indexArg)
	if err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	encBytes, err := utils.ParseByteArray(encOutArg)
	if err != nil {
		return nil, fmt.Errorf("parse encrypted_output: %w", err)
	}

	encryptedOutput := "0x" + hex.EncodeToString(encBytes)
	onChainArg, err := ev.GetArg("on_chain_data")
	if err != nil {
		return &types.CommitmentEvent{Commitment: commitment, Index: index, EncryptedOutput: encryptedOutput}, nil
	}
	onChainData, parseErr := utils.ParseByteMatrix(onChainArg)
	if parseErr != nil {
		return nil, fmt.Errorf("parse on_chain_data: %w", parseErr)
	}
	if hasOnChainData(onChainData) {
		encodedOutput, encodeErr := utils.EncodeSolanaOnChainUtxo(onChainData)
		if encodeErr != nil {
			return nil, encodeErr
		}
		event := &types.CommitmentEvent{Commitment: commitment, Index: index, EncryptedOutput: encodedOutput}
		if len(encBytes) > 0 {
			event.EncryptedBlob = encryptedOutput
		}
		return event, nil
	}
	return &types.CommitmentEvent{Commitment: commitment, Index: index, EncryptedOutput: encryptedOutput}, nil
}

func (s *SolanaCommitmentsSnapshotService) AcceptEvent(
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
		Value:         event.EncryptedOutput,
		IsPositive:    !utils.IsSolanaOnChainUtxo(event.EncryptedOutput),
		IsBlocked:     isBlocked,
		EncryptedBlob: event.EncryptedBlob,
	})
	return true
}
