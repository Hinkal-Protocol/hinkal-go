package tests

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/snapshot"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func solanaRPC() string {
	if url, err := constants.FetchRPCURL(constants.ChainIDs.SolanaMainnet); err == nil {
		return url
	}
	return "https://api.mainnet-beta.solana.com"
}

var (
	discNewCommitment = [8]byte{78, 21, 75, 243, 5, 132, 204, 67}
	discNullified     = [8]byte{124, 69, 200, 64, 75, 203, 121, 17}
)

func anchorPayload(disc [8]byte, fields []byte) []byte {
	return append(append([]byte{}, disc[:]...), fields...)
}

func programDataLog(disc [8]byte, fields []byte) string {
	return "Program data: " + base64.StdEncoding.EncodeToString(anchorPayload(disc, fields))
}

func cpiInstructionData(disc [8]byte, fields []byte) string {
	raw := append(make([]byte, 8), anchorPayload(disc, fields)...)
	return base58.Encode(raw)
}

func newCommitmentFields(commitment, index *big.Int, encrypted []byte) []byte {
	b := commitment.FillBytes(make([]byte, 32))
	b = append(b, index.FillBytes(make([]byte, 32))...)
	encLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(encLen, uint32(len(encrypted)))
	b = append(b, encLen...)
	b = append(b, encrypted...)
	b = append(b, make([]byte, 9*32)...)
	return b
}

func TestSolanaDecodeNewCommitment_ToService(t *testing.T) {
	commitment := big.NewInt(1111)
	fields := newCommitmentFields(commitment, leaf(0), []byte{0xaa, 0xbb})

	logs := solana.ParseLogsForEvents([]string{programDataLog(discNewCommitment, fields)})
	cpi := solana.ParseCpiForEvents([]string{cpiInstructionData(discNewCommitment, fields)})
	for name, set := range map[string][]*solana.DecodedEvent{"logs": logs, "cpi": cpi} {
		if len(set) != 1 || set[0].Name != "NewCommitment" {
			t.Fatalf("%s decoded = %+v", name, set)
		}
	}

	f := newSolanaCommitmentsFixture(t, 5)
	process(t, f.emitter, 11, mkEvent(t, logs[0].Name, "0xsig", 10, logs[0].Args))

	assertSolanaLeaf(t, f.svc, leaf(0), commitment)
	assertEqual(t, "decoded off-chain output", solanaOutput(f.svc, "0xaabb"),
		types.EncryptedOutputWithSign{Value: "0xaabb", IsPositive: true, IsBlocked: false})
}

func TestSolanaDecodeNullified_ToService(t *testing.T) {
	fields := big.NewInt(255).FillBytes(make([]byte, 32))

	logs := solana.ParseLogsForEvents([]string{programDataLog(discNullified, fields)})
	cpi := solana.ParseCpiForEvents([]string{cpiInstructionData(discNullified, fields)})
	for name, set := range map[string][]*solana.DecodedEvent{"logs": logs, "cpi": cpi} {
		if len(set) != 1 || set[0].Name != "Nullified" {
			t.Fatalf("%s decoded = %+v", name, set)
		}
	}

	f := newSolanaNullifierFixture(t, 5)
	process(t, f.emitter, 6, mkEvent(t, cpi[0].Name, "0xsig", 6, cpi[0].Args))
	assertSolanaNullifiers(t, f.svc, "0xff")
}

func TestSolanaBlockchainEventEmitter_SyncFromAtMost_FirstCallWins(t *testing.T) {
	e := eventservice.NewSolanaBlockchainEventEmitter(constants.ChainIDs.SolanaMainnet, "", "prog", 0, false, nil, nil)
	e.SyncFromAtMost(5)
	assertEqual(t, "after first sync", e.LatestBlockNumber(), uint64(5))

	e.SyncFromAtMost(8)
	assertEqual(t, "after higher sync", e.LatestBlockNumber(), uint64(5))

	e.SyncFromAtMost(3)
	assertEqual(t, "after lower sync", e.LatestBlockNumber(), uint64(3))
}

func TestSolanaDecodeFromChain_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const programID = "8ZWGr2cv8uhDBUwc62NRDFYpz4zWgiLUxGGyd8dWhCa8"
	client := solana.NewClient(solanaRPC())
	sigs, err := client.GetSignaturesForAddress(ctx, programID, 100, "")
	if err != nil {
		t.Fatalf("get signatures: %v", err)
	}

	counts := map[string]int{}
	for _, sig := range sigs {
		if sig.Err != nil {
			continue
		}
		tx, err := client.GetTransaction(ctx, sig.Signature)
		if err != nil || tx == nil || tx.Meta == nil || tx.Meta.Err != nil {
			continue
		}
		for _, ev := range solana.ParseLogsForEvents(tx.Meta.LogMessages) {
			counts["log:"+ev.Name]++
		}
		var cpiData []string
		for _, inner := range tx.Meta.InnerInstructions {
			for _, ix := range inner.Instructions {
				if ix.Data != "" {
					cpiData = append(cpiData, ix.Data)
				}
			}
		}
		for _, ev := range solana.ParseCpiForEvents(cpiData) {
			counts["cpi:"+ev.Name]++
		}
	}

	t.Logf("decoded from last %d signatures: %+v", len(sigs), counts)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		t.Fatal("decoded zero events from real chain transactions (decoder or RPC parsing broken)")
	}
}

func TestSolanaClientCommitmentsSnapshot_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	chainID := constants.ChainIDs.SolanaMainnet
	raw, err := api.FetchSnapshots(ctx, chainID)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}
	programID := raw.Commitments.HinkalAddress

	fetcher := snapshot.NewSnapshotFetcherService(chainID, programID)
	emitter := eventservice.NewSolanaBlockchainEventEmitter(chainID, solanaRPC(), programID, 0, false, nil, nil)
	commitments := snapshot.NewClientSolanaCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
	nullifiers := snapshot.NewClientSolanaNullifierSnapshotService(emitter, fetcher)

	if err := commitments.Svc.Init(ctx); err != nil {
		t.Fatalf("init commitments: %v", err)
	}
	if err := nullifiers.Svc.Init(ctx); err != nil {
		t.Fatalf("init nullifiers: %v", err)
	}

	snapshotRoot, err := commitments.MerkleTree().GetRootHash()
	if err != nil {
		t.Fatalf("root after snapshot load: %v", err)
	}

	accepted := 0
	emitter.OnEventsProcessed = func(n int) { accepted += n }
	if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
		t.Fatalf("gap-fill from chain: %v", err)
	}

	root, err := commitments.MerkleTree().GetRootHash()
	if err != nil {
		t.Fatalf("root after gap-fill: %v", err)
	}
	if root.Sign() == 0 {
		t.Fatal("computed root is zero")
	}
	t.Logf("solana: snapSlot=%d nodes=%d nullifiers=%d gapEvents=%d snapRoot=0x%s root=0x%s",
		raw.Commitments.LatestBlockNumber, len(raw.Commitments.MerkleTree.Tree),
		len(nullifiers.Nullifiers()), accepted, snapshotRoot.Text(16), root.Text(16))
}
