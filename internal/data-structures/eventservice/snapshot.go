package eventservice

import (
	"context"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
)

type Snapshot[P any] struct {
	LatestBlockNumber uint64
	Payload           P
}

type EventEmitter interface {
	AddEventProcessorFunction(fn blockchainevent.EventProcessorFunc)
	RetrieveEvents(ctx context.Context, fromBlock uint64, force bool) error
	LatestBlockNumber() uint64
	SyncFromAtMost(blockNumber uint64)
	IsReady() bool
	ChainID() int
	IntervalClear()
}

type SnapshotDelegate[E, P, S any] interface {
	AcceptEvent(event E, blockNumber uint64, txHash string, isBlocked bool) bool
	MapEvent(event *blockchainevent.BlockchainEvent) (E, error)
	GetSnapshotPayload() P
	PopulateSnapshot(snap Snapshot[P])
	SerializeSnapshot(snap Snapshot[P]) (S, error)
	DeserializeSnapshot(serialized S) (Snapshot[P], error)
	FetchSnapshot(ctx context.Context) (S, error)
	PersistSnapshot(ctx context.Context, serialized S) error
}

type SnapshotService[E, P, S any] struct {
	Emitter          EventEmitter
	eventName        string
	savedLatestBlock uint64
	delegate         SnapshotDelegate[E, P, S]
}

func NewSnapshotService[E, P, S any](
	emitter EventEmitter,
	eventName string,
	delegate SnapshotDelegate[E, P, S],
) *SnapshotService[E, P, S] {
	svc := &SnapshotService[E, P, S]{
		Emitter:   emitter,
		eventName: eventName,
		delegate:  delegate,
	}
	emitter.AddEventProcessorFunction(svc.processEventsPage)
	return svc
}

func (s *SnapshotService[E, P, S]) Init(ctx context.Context) error {
	return s.loadSnapshot(ctx)
}

func (s *SnapshotService[E, P, S]) RetrieveEventsFromLatestBlock(ctx context.Context) error {
	return s.Emitter.RetrieveEvents(ctx, s.Emitter.LatestBlockNumber(), false)
}

func (s *SnapshotService[E, P, S]) IntervalClear() { s.Emitter.IntervalClear() }

func (s *SnapshotService[E, P, S]) IsReady() bool { return s.Emitter.IsReady() }

func (s *SnapshotService[E, P, S]) loadSnapshot(ctx context.Context) error {
	serialized, err := s.delegate.FetchSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("fetch snapshot: %w", err)
	}
	snap, err := s.delegate.DeserializeSnapshot(serialized)
	if err != nil {
		return fmt.Errorf("deserialize snapshot: %w", err)
	}
	s.delegate.PopulateSnapshot(snap)
	s.Emitter.SyncFromAtMost(snap.LatestBlockNumber)
	s.savedLatestBlock = snap.LatestBlockNumber
	return nil
}

func (s *SnapshotService[E, P, S]) SaveSnapshot(ctx context.Context) error {
	s.savedLatestBlock = s.Emitter.LatestBlockNumber()
	snap := Snapshot[P]{
		LatestBlockNumber: s.Emitter.LatestBlockNumber(),
		Payload:           s.delegate.GetSnapshotPayload(),
	}
	serialized, err := s.delegate.SerializeSnapshot(snap)
	if err != nil {
		return fmt.Errorf("serialize snapshot: %w", err)
	}
	return s.delegate.PersistSnapshot(ctx, serialized)
}

func (s *SnapshotService[E, P, S]) afterEventsAccepted(ctx context.Context, count int) error {
	depth, err := constants.GetSaveDepth(s.Emitter.ChainID())
	if err != nil {
		return err
	}
	if count > 0 || s.savedLatestBlock+depth < s.Emitter.LatestBlockNumber() {
		return s.SaveSnapshot(ctx)
	}
	return nil
}

func (s *SnapshotService[E, P, S]) processEventsPage(events []*blockchainevent.BlockchainEvent, _ *uint64) (int, error) {
	blockedTxHashes := make(map[string]bool)
	for _, ev := range events {
		if ev.EventName == "BlockedUtxosCreated" {
			blockedTxHashes[ev.TransactionHash] = true
		}
	}
	accepted := 0
	for _, ev := range events {
		if ev.EventName != s.eventName {
			continue
		}
		mapped, err := s.delegate.MapEvent(ev)
		if err != nil {
			continue
		}
		isBlocked := blockedTxHashes[ev.TransactionHash]
		if s.delegate.AcceptEvent(mapped, ev.BlockNumber, ev.TransactionHash, isBlocked) {
			accepted++
		}
	}
	return accepted, s.afterEventsAccepted(context.Background(), accepted)
}
