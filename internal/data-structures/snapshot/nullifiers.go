package snapshot

import (
	"context"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type NullifiersPayload struct {
	Nullifiers map[string]struct{}
}

type NullifierSnapshotService struct {
	Svc *eventservice.SnapshotService[
		*types.NullifierEvent,
		*NullifiersPayload,
		*types.NullifierSerializedSnapshot,
	]
	nullifiers map[string]struct{}
	fetchFn    func(ctx context.Context) (*types.NullifierSerializedSnapshot, error)
	persistFn  func(ctx context.Context, s *types.NullifierSerializedSnapshot) error
}

func NewNullifierSnapshotService(
	emitter eventservice.EventEmitter,
	fetchFn func(ctx context.Context) (*types.NullifierSerializedSnapshot, error),
	persistFn func(ctx context.Context, s *types.NullifierSerializedSnapshot) error,
) *NullifierSnapshotService {
	svc := &NullifierSnapshotService{
		nullifiers: make(map[string]struct{}),
		fetchFn:    fetchFn,
		persistFn:  persistFn,
	}
	svc.Svc = eventservice.NewSnapshotService(emitter, "Nullified", svc)
	return svc
}

func NewClientNullifierSnapshotService(
	emitter eventservice.EventEmitter,
	fetcher *SnapshotFetcherService,
) *NullifierSnapshotService {
	return NewNullifierSnapshotService(
		emitter,
		func(ctx context.Context) (*types.NullifierSerializedSnapshot, error) {
			return fetcher.GetNullifiers(ctx)
		},
		func(_ context.Context, _ *types.NullifierSerializedSnapshot) error { return nil },
	)
}

func (s *NullifierSnapshotService) Nullifiers() map[string]struct{} { return s.nullifiers }

func (s *NullifierSnapshotService) IntervalClear() { s.Svc.IntervalClear() }

func (s *NullifierSnapshotService) FetchSnapshot(ctx context.Context) (*types.NullifierSerializedSnapshot, error) {
	return s.fetchFn(ctx)
}

func (s *NullifierSnapshotService) PersistSnapshot(ctx context.Context, snap *types.NullifierSerializedSnapshot) error {
	return s.persistFn(ctx, snap)
}

func (s *NullifierSnapshotService) GetSnapshotPayload() *NullifiersPayload {
	return &NullifiersPayload{Nullifiers: s.nullifiers}
}

func (s *NullifierSnapshotService) PopulateSnapshot(snap eventservice.Snapshot[*NullifiersPayload]) {
	s.nullifiers = snap.Payload.Nullifiers
}

func (s *NullifierSnapshotService) SerializeSnapshot(
	snap eventservice.Snapshot[*NullifiersPayload],
) (*types.NullifierSerializedSnapshot, error) {
	block := snap.LatestBlockNumber
	nullifiers := make([]string, 0, len(snap.Payload.Nullifiers))
	for n := range snap.Payload.Nullifiers {
		nullifiers = append(nullifiers, n)
	}
	return &types.NullifierSerializedSnapshot{LatestBlockNumber: &block, Nullifiers: nullifiers}, nil
}

func (s *NullifierSnapshotService) DeserializeSnapshot(
	serialized *types.NullifierSerializedSnapshot,
) (eventservice.Snapshot[*NullifiersPayload], error) {
	nullifiers := make(map[string]struct{}, len(serialized.Nullifiers))
	for _, n := range serialized.Nullifiers {
		nullifiers[n] = struct{}{}
	}
	latestBlock := uint64(0)
	if serialized.LatestBlockNumber != nil {
		latestBlock = *serialized.LatestBlockNumber
	}
	return eventservice.Snapshot[*NullifiersPayload]{
		LatestBlockNumber: latestBlock,
		Payload:           &NullifiersPayload{Nullifiers: nullifiers},
	}, nil
}

func (s *NullifierSnapshotService) MapEvent(ev *blockchainevent.BlockchainEvent) (*types.NullifierEvent, error) {
	nullifierStr, err := ev.GetArg("nullifier")
	if err != nil {
		return nil, err
	}
	n, err := utils.ParseBigInt(nullifierStr)
	if err != nil {
		return nil, fmt.Errorf("parse nullifier: %w", err)
	}
	return &types.NullifierEvent{Nullifier: utils.ToBeHex(n)}, nil
}

func (s *NullifierSnapshotService) AcceptEvent(event *types.NullifierEvent, _ uint64, _ string, _ bool) bool {
	if _, exists := s.nullifiers[event.Nullifier]; exists {
		return false
	}
	s.nullifiers[event.Nullifier] = struct{}{}
	return true
}
