package tests

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/snapshot"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type noopDelegate struct{}

func (noopDelegate) Clear()                                                                    {}
func (noopDelegate) StartUpdateListener(context.Context, *eventservice.BlockchainEventEmitter) {}

func newTestEmitter() *eventservice.BlockchainEventEmitter {
	return eventservice.New(constants.ChainIDs.EthMainnet, nil, common.Address{}, abi.ABI{}, 0, false, noopDelegate{}, nil, nil)
}

func leaf(offset int64) *big.Int {
	return new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 199), big.NewInt(offset))
}

func neg(n *big.Int) *big.Int { return new(big.Int).Neg(n) }

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func assertEqual[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %+v, want %+v", what, got, want)
	}
}

func mkEvent(t *testing.T, name, tx string, block uint64, args map[string]any) *blockchainevent.BlockchainEvent {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"eventName":       name,
		"transactionHash": tx,
		"blockNumber":     block,
		"args":            args,
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	ev, err := blockchainevent.NewFromSerialized(string(b))
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	return ev
}

func commitmentEvent(t *testing.T, tx string, block uint64, commitment, index *big.Int, encryptedOutput string) *blockchainevent.BlockchainEvent {
	return mkEvent(t, "NewCommitment", tx, block, map[string]any{
		"commitment":      commitment.String(),
		"index":           index.String(),
		"encryptedOutput": encryptedOutput,
	})
}

func blockedEvent(t *testing.T, tx string, block uint64) *blockchainevent.BlockchainEvent {
	return mkEvent(t, "BlockedUtxosCreated", tx, block, map[string]any{})
}

func nullifierEvent(t *testing.T, tx string, block uint64, nullifier string) *blockchainevent.BlockchainEvent {
	return mkEvent(t, "Nullified", tx, block, map[string]any{"nullifier": nullifier})
}

func process(t *testing.T, e *eventservice.BlockchainEventEmitter, block uint64, events ...*blockchainevent.BlockchainEvent) {
	t.Helper()
	if err := e.ProcessExternalEvents(events, block); err != nil {
		t.Fatalf("process events: %v", err)
	}
}

type commitmentsFixture struct {
	svc     *snapshot.CommitmentsSnapshotService
	emitter *eventservice.BlockchainEventEmitter
	saved   []*types.CommitmentsSerializedSnapshot
}

func newCommitmentsFixture(t *testing.T, startBlock uint64) *commitmentsFixture {
	t.Helper()
	f := &commitmentsFixture{emitter: newTestEmitter()}
	f.svc = snapshot.NewCommitmentsSnapshotService(
		f.emitter, crypto.PoseidonHashFunc,
		func(context.Context) (*types.CommitmentsSerializedSnapshot, error) {
			block := startBlock
			return &types.CommitmentsSerializedSnapshot{LatestBlockNumber: &block}, nil
		},
		func(_ context.Context, s *types.CommitmentsSerializedSnapshot) error {
			f.saved = append(f.saved, s)
			return nil
		},
	)
	if err := f.svc.Svc.Init(context.Background()); err != nil {
		t.Fatalf("init commitments service: %v", err)
	}
	return f
}

func (f *commitmentsFixture) lastSaved(t *testing.T) *types.CommitmentsSerializedSnapshot {
	t.Helper()
	if len(f.saved) == 0 {
		t.Fatal("no snapshot persisted")
	}
	return f.saved[len(f.saved)-1]
}

func assertLeaf(t *testing.T, svc *snapshot.CommitmentsSnapshotService, index, want *big.Int) {
	t.Helper()
	got, present := svc.MerkleTree().GetValue(index)
	if !present || got.Cmp(want) != 0 {
		t.Fatalf("leaf[%v] = %v (present=%v), want %v", index, got, present, want)
	}
}

func liveOutput(svc *snapshot.CommitmentsSnapshotService, value string) types.EncryptedOutputWithSign {
	for _, o := range svc.EncryptedOutputs() {
		if o.Value == value {
			return *o
		}
	}
	return types.EncryptedOutputWithSign{}
}

func savedOutput(s *types.CommitmentsSerializedSnapshot, value string) types.SerializedEncryptedOutputWithSign {
	for _, o := range s.EncryptedOutputs {
		if o.Value == value {
			return o
		}
	}
	return types.SerializedEncryptedOutputWithSign{}
}

type nullifierFixture struct {
	svc     *snapshot.NullifierSnapshotService
	emitter *eventservice.BlockchainEventEmitter
}

func newNullifierFixture(t *testing.T, startBlock uint64, preloaded ...string) *nullifierFixture {
	t.Helper()
	f := &nullifierFixture{emitter: newTestEmitter()}
	f.svc = snapshot.NewNullifierSnapshotService(
		f.emitter,
		func(context.Context) (*types.NullifierSerializedSnapshot, error) {
			block := startBlock
			return &types.NullifierSerializedSnapshot{LatestBlockNumber: &block, Nullifiers: preloaded}, nil
		},
		func(context.Context, *types.NullifierSerializedSnapshot) error { return nil },
	)
	if err := f.svc.Svc.Init(context.Background()); err != nil {
		t.Fatalf("init nullifier service: %v", err)
	}
	return f
}

func assertNullifiers(t *testing.T, svc *snapshot.NullifierSnapshotService, want ...string) {
	t.Helper()
	got := svc.Nullifiers()
	have := make([]string, 0, len(got))
	for k := range got {
		have = append(have, k)
	}
	if len(got) != len(want) {
		t.Fatalf("nullifiers = %v, want %v", have, want)
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Fatalf("missing nullifier %q in %v", w, have)
		}
	}
}

func TestCommitmentsSnapshotService_EndToEnd(t *testing.T) {
	f := newCommitmentsFixture(t, 5)
	c1, c2 := big.NewInt(1111), big.NewInt(2222)

	process(t, f.emitter, 11,
		commitmentEvent(t, "0xtx1", 10, c1, leaf(0), "0xaa"),
		commitmentEvent(t, "0xtx2", 11, c2, neg(leaf(1)), "0xbb"),
		blockedEvent(t, "0xtx2", 11),
	)

	assertLeaf(t, f.svc, leaf(0), c1)
	assertLeaf(t, f.svc, leaf(1), c2)

	assertEqual(t, "0xaa output", liveOutput(f.svc, "0xaa"),
		types.EncryptedOutputWithSign{Value: "0xaa", IsPositive: true, IsBlocked: false})
	assertEqual(t, "0xbb output", liveOutput(f.svc, "0xbb"),
		types.EncryptedOutputWithSign{Value: "0xbb", IsPositive: false, IsBlocked: true})

	if _, err := f.svc.MerkleTree().GetRootHash(); err != nil {
		t.Fatalf("root hash: %v", err)
	}

	last := f.lastSaved(t)
	assertEqual(t, "persisted block", deref(last.LatestBlockNumber), uint64(5))
	assertEqual(t, "emitter block after batch", f.emitter.LatestBlockNumber(), uint64(11))
	assertEqual(t, "serialized 0xbb output", savedOutput(last, "0xbb"),
		types.SerializedEncryptedOutputWithSign{Value: "0xbb", IsPositive: "false", IsBlocked: true})
}

func TestCommitmentsSnapshotService_DedupAndReplace(t *testing.T) {
	f := newCommitmentsFixture(t, 0)
	original, replacement := big.NewInt(1111), big.NewInt(9999)

	process(t, f.emitter, 10, commitmentEvent(t, "0xtx1", 10, original, leaf(0), "0xaa"))
	process(t, f.emitter, 10, commitmentEvent(t, "0xtx1", 10, original, leaf(0), "0xaa"))
	assertEqual(t, "outputs after duplicate", len(f.svc.EncryptedOutputs()), 1)

	process(t, f.emitter, 11, commitmentEvent(t, "0xtx2", 11, replacement, leaf(0), "0xcc"))
	assertLeaf(t, f.svc, leaf(0), replacement)
	assertEqual(t, "outputs after replace", len(f.svc.EncryptedOutputs()), 1)
	assertEqual(t, "remaining output", f.svc.EncryptedOutputs()[0].Value, "0xcc")
}

func TestNullifierSnapshotService_ToBeHexParity(t *testing.T) {
	f := newNullifierFixture(t, 5, "0x0a")

	process(t, f.emitter, 6, nullifierEvent(t, "0xtx1", 6, "10"))
	assertNullifiers(t, f.svc, "0x0a")

	process(t, f.emitter, 8,
		nullifierEvent(t, "0xtx2", 7, "255"),
		nullifierEvent(t, "0xtx3", 8, "256"),
	)
	assertNullifiers(t, f.svc, "0x0a", "0xff", "0x0100")
}

func TestBlockchainEventEmitter_SyncFromAtMost_FirstCallWins(t *testing.T) {
	e := newTestEmitter()
	e.SyncFromAtMost(5)
	assertEqual(t, "after first sync", e.LatestBlockNumber(), uint64(5))

	e.SyncFromAtMost(8)
	assertEqual(t, "after higher sync", e.LatestBlockNumber(), uint64(5))

	e.SyncFromAtMost(3)
	assertEqual(t, "after lower sync", e.LatestBlockNumber(), uint64(3))
}

func TestFetchSnapshots_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	snaps, err := api.FetchSnapshots(ctx, constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}
	assertEqual(t, "commitments chainId", snaps.Commitments.ChainID, constants.ChainIDs.EthMainnet)
	if snaps.Commitments.HinkalAddress == "" {
		t.Fatal("empty hinkalAddress")
	}
	if len(snaps.Commitments.MerkleTree.Tree) == 0 {
		t.Fatal("empty merkle tree")
	}
	t.Logf("block=%d nodes=%d nullifiers=%d",
		snaps.Commitments.LatestBlockNumber, len(snaps.Commitments.MerkleTree.Tree), len(snaps.Nullifiers.Nullifiers))
}

func TestClientCommitmentsSnapshot_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.EthMainnet
	raw, err := api.FetchSnapshots(ctx, chainID)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}

	fetcher := snapshot.NewSnapshotFetcherService(chainID, raw.Commitments.HinkalAddress)
	svc := snapshot.NewClientCommitmentsSnapshotService(newTestEmitter(), crypto.PoseidonHashFunc, fetcher)
	if err := svc.Svc.Init(ctx); err != nil {
		t.Fatalf("init from live snapshot: %v", err)
	}

	root, err := svc.MerkleTree().GetRootHash()
	if err != nil {
		t.Fatalf("root hash: %v", err)
	}
	if root.Sign() == 0 {
		t.Fatal("computed root is zero")
	}
	t.Logf("block=%d nodes=%d root=0x%s",
		raw.Commitments.LatestBlockNumber, len(raw.Commitments.MerkleTree.Tree), root.Text(16))
}
