package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/snapshot"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func bytes32(n *big.Int) []int {
	b := n.FillBytes(make([]byte, 32))
	out := make([]int, 32)
	for i, v := range b {
		out[i] = int(v)
	}
	return out
}

func byteInts(b ...byte) []int {
	out := make([]int, len(b))
	for i, v := range b {
		out[i] = int(v)
	}
	return out
}

func onChainFields() [][]int {
	rows := make([][]int, 9)
	for i := range rows {
		row := make([]int, 32)
		for j := range row {
			row[j] = (i*32 + j) % 256
		}
		rows[i] = row
	}
	return rows
}

func toBytesMatrix(rows [][]int) [][]byte {
	out := make([][]byte, len(rows))
	for i, r := range rows {
		b := make([]byte, len(r))
		for j, v := range r {
			b[j] = byte(v)
		}
		out[i] = b
	}
	return out
}

func solanaCommitmentEvent(t *testing.T, tx string, block uint64, commitment, index *big.Int, encryptedOutput []int, onChainData [][]int) *blockchainevent.BlockchainEvent {
	args := map[string]any{
		"commitment":       bytes32(commitment),
		"index":            bytes32(index),
		"encrypted_output": encryptedOutput,
	}
	if onChainData != nil {
		args["on_chain_data"] = onChainData
	}
	return mkEvent(t, "NewCommitment", tx, block, args)
}

func solanaNullifierEvent(t *testing.T, tx string, block uint64, value *big.Int) *blockchainevent.BlockchainEvent {
	return mkEvent(t, "Nullified", tx, block, map[string]any{"nullifier": bytes32(value)})
}

type solanaCommitmentsFixture struct {
	svc     *snapshot.SolanaCommitmentsSnapshotService
	emitter *eventservice.BlockchainEventEmitter
	saved   []*types.CommitmentsSerializedSnapshot
}

func newSolanaCommitmentsFixture(t *testing.T, startBlock uint64) *solanaCommitmentsFixture {
	t.Helper()
	f := &solanaCommitmentsFixture{emitter: newTestEmitter()}
	f.svc = snapshot.NewSolanaCommitmentsSnapshotService(
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
		t.Fatalf("init solana commitments service: %v", err)
	}
	return f
}

type solanaNullifierFixture struct {
	svc     *snapshot.SolanaNullifierSnapshotService
	emitter *eventservice.BlockchainEventEmitter
}

func newSolanaNullifierFixture(t *testing.T, startBlock uint64, preloaded ...string) *solanaNullifierFixture {
	t.Helper()
	f := &solanaNullifierFixture{emitter: newTestEmitter()}
	f.svc = snapshot.NewSolanaNullifierSnapshotService(
		f.emitter,
		func(context.Context) (*types.NullifierSerializedSnapshot, error) {
			block := startBlock
			return &types.NullifierSerializedSnapshot{LatestBlockNumber: &block, Nullifiers: preloaded}, nil
		},
		func(context.Context, *types.NullifierSerializedSnapshot) error { return nil },
	)
	if err := f.svc.Svc.Init(context.Background()); err != nil {
		t.Fatalf("init solana nullifier service: %v", err)
	}
	return f
}

func assertSolanaLeaf(t *testing.T, svc *snapshot.SolanaCommitmentsSnapshotService, index, want *big.Int) {
	t.Helper()
	got, present := svc.MerkleTree().GetValue(index)
	if !present || got.Cmp(want) != 0 {
		t.Fatalf("leaf[%v] = %v (present=%v), want %v", index, got, present, want)
	}
}

func solanaOutput(svc *snapshot.SolanaCommitmentsSnapshotService, value string) types.EncryptedOutputWithSign {
	for _, o := range svc.EncryptedOutputs() {
		if o.Value == value {
			return *o
		}
	}
	return types.EncryptedOutputWithSign{}
}

func assertSolanaNullifiers(t *testing.T, svc *snapshot.SolanaNullifierSnapshotService, want ...string) {
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

func TestSolanaCommitmentsSnapshotService_EndToEnd(t *testing.T) {
	f := newSolanaCommitmentsFixture(t, 5)
	c1, c2 := big.NewInt(1111), big.NewInt(2222)
	onChain := onChainFields()

	process(t, f.emitter, 11,
		solanaCommitmentEvent(t, "0xtx1", 10, c1, leaf(0), byteInts(0xaa, 0xbb), nil),
		solanaCommitmentEvent(t, "0xtx2", 11, c2, leaf(1), byteInts(0xcc, 0xdd), onChain),
		blockedEvent(t, "0xtx2", 11),
	)

	assertSolanaLeaf(t, f.svc, leaf(0), c1)
	assertSolanaLeaf(t, f.svc, leaf(1), c2)

	assertEqual(t, "off-chain output", solanaOutput(f.svc, "0xaabb"),
		types.EncryptedOutputWithSign{Value: "0xaabb", IsPositive: true, IsBlocked: false})

	encoded, err := utils.EncodeSolanaOnChainUtxo(toBytesMatrix(onChain))
	if err != nil {
		t.Fatalf("encode on-chain utxo: %v", err)
	}
	assertEqual(t, "on-chain output", solanaOutput(f.svc, encoded),
		types.EncryptedOutputWithSign{Value: encoded, IsPositive: false, IsBlocked: true, EncryptedBlob: "0xccdd"})

	if _, err := f.svc.MerkleTree().GetRootHash(); err != nil {
		t.Fatalf("root hash: %v", err)
	}
}

func TestSolanaNullifierSnapshotService_ToBeHexParity(t *testing.T) {
	f := newSolanaNullifierFixture(t, 5, "0x0a")

	process(t, f.emitter, 6, solanaNullifierEvent(t, "0xtx1", 6, big.NewInt(10)))
	assertSolanaNullifiers(t, f.svc, "0x0a")

	process(t, f.emitter, 8,
		solanaNullifierEvent(t, "0xtx2", 7, big.NewInt(255)),
		solanaNullifierEvent(t, "0xtx3", 8, big.NewInt(256)),
	)
	assertSolanaNullifiers(t, f.svc, "0x0a", "0xff", "0x0100")
}
