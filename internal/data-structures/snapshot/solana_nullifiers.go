package snapshot

import (
	"context"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SolanaNullifierSnapshotService struct {
	Svc *eventservice.SnapshotService[
		*types.NullifierEvent,
		*NullifiersPayload,
		*types.NullifierSerializedSnapshot,
	]
	nullifiers map[string]struct{}
	fetchFn    func(ctx context.Context) (*types.NullifierSerializedSnapshot, error)
	persistFn  func(ctx context.Context, s *types.NullifierSerializedSnapshot) error
}

func NewSolanaNullifierSnapshotService(
	emitter eventservice.EventEmitter,
	fetchFn func(ctx context.Context) (*types.NullifierSerializedSnapshot, error),
	persistFn func(ctx context.Context, s *types.NullifierSerializedSnapshot) error,
) *SolanaNullifierSnapshotService {
	svc := &SolanaNullifierSnapshotService{
		nullifiers: make(map[string]struct{}),
		fetchFn:    fetchFn,
		persistFn:  persistFn,
	}
	svc.Svc = eventservice.NewSnapshotService(emitter, "Nullified", svc)
	return svc
}

func NewClientSolanaNullifierSnapshotService(
	emitter eventservice.EventEmitter,
	fetcher *SnapshotFetcherService,
) *SolanaNullifierSnapshotService {
	return NewSolanaNullifierSnapshotService(
		emitter,
		func(ctx context.Context) (*types.NullifierSerializedSnapshot, error) {
			return fetcher.GetNullifiers(ctx)
		},
		func(_ context.Context, _ *types.NullifierSerializedSnapshot) error { return nil },
	)
}

func (s *SolanaNullifierSnapshotService) Nullifiers() map[string]struct{} { return s.nullifiers }

func (s *SolanaNullifierSnapshotService) IntervalClear() { s.Svc.IntervalClear() }

func (s *SolanaNullifierSnapshotService) FetchSnapshot(ctx context.Context) (*types.NullifierSerializedSnapshot, error) {
	return s.fetchFn(ctx)
}

func (s *SolanaNullifierSnapshotService) PersistSnapshot(ctx context.Context, snap *types.NullifierSerializedSnapshot) error {
	return s.persistFn(ctx, snap)
}

func (s *SolanaNullifierSnapshotService) GetSnapshotPayload() *NullifiersPayload {
	return &NullifiersPayload{Nullifiers: s.nullifiers}
}

func (s *SolanaNullifierSnapshotService) PopulateSnapshot(snap eventservice.Snapshot[*NullifiersPayload]) {
	s.nullifiers = snap.Payload.Nullifiers
}

func (s *SolanaNullifierSnapshotService) SerializeSnapshot(
	snap eventservice.Snapshot[*NullifiersPayload],
) (*types.NullifierSerializedSnapshot, error) {
	block := snap.LatestBlockNumber
	nullifiers := make([]string, 0, len(snap.Payload.Nullifiers))
	for n := range snap.Payload.Nullifiers {
		nullifiers = append(nullifiers, n)
	}
	return &types.NullifierSerializedSnapshot{LatestBlockNumber: &block, Nullifiers: nullifiers}, nil
}

func (s *SolanaNullifierSnapshotService) DeserializeSnapshot(
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

func (s *SolanaNullifierSnapshotService) MapEvent(ev *blockchainevent.BlockchainEvent) (*types.NullifierEvent, error) {
	arg, err := ev.GetArg("nullifier")
	if err != nil {
		return nil, err
	}
	n, err := utils.AdvancedToBigInt(arg)
	if err != nil {
		return nil, fmt.Errorf("parse nullifier: %w", err)
	}
	return &types.NullifierEvent{Nullifier: utils.ToBeHex(n)}, nil
}

func (s *SolanaNullifierSnapshotService) AcceptEvent(event *types.NullifierEvent, _ uint64, _ string, _ bool) bool {
	if _, exists := s.nullifiers[event.Nullifier]; exists {
		return false
	}
	s.nullifiers[event.Nullifier] = struct{}{}
	return true
}
