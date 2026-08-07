package tests

import (
	"math/big"
	"testing"

	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
)

// Layout offsets independently encode the Merkle account IDL spec
// (disc 8 + version 1 + tree_levels 200x32 + roots 100x32 + m_index 32 + root_index 32 + ...).
const (
	testMerkleRootsOffset     = 8 + 1 + 200*32                      // 6409
	testMerkleRootIndexOffset = testMerkleRootsOffset + 100*32 + 32 // 9641
	testMerkleAccountLen      = testMerkleRootIndexOffset + 32 + 16 + 32
)

var testMerkleDiscriminator = []byte{55, 52, 102, 252, 195, 69, 204, 210}

func TestParseMerkleRootHash(t *testing.T) {
	data := make([]byte, testMerkleAccountLen)
	copy(data[:8], testMerkleDiscriminator)

	// root_index = 5 (big-endian) -> latest root lives at roots[4]
	data[testMerkleRootIndexOffset+31] = 5
	want := big.NewInt(0xBEEF)
	rootStart := testMerkleRootsOffset + 4*32
	want.FillBytes(data[rootStart : rootStart+32])

	got, err := solana.ParseMerkleRootHash(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("root = %s, want %s", got, want)
	}
}

func TestParseMerkleRootHash_BadDiscriminator(t *testing.T) {
	if _, err := solana.ParseMerkleRootHash(make([]byte, testMerkleAccountLen)); err == nil {
		t.Fatal("expected error for wrong discriminator")
	}
}

func TestParseMerkleRootHash_TooShort(t *testing.T) {
	if _, err := solana.ParseMerkleRootHash(make([]byte, 10)); err == nil {
		t.Fatal("expected error for short account data")
	}
}

func TestGetMerkleAccountPublicKey(t *testing.T) {
	const programID = "8ZWGr2cv8uhDBUwc62NRDFYpz4zWgiLUxGGyd8dWhCa8"
	const originalDeployer = "8xzyYDbvfpqYEGekNsm6KDyNXJybTLJhBWBQwabsZN49"

	deployerBytes, err := base58.Decode(originalDeployer)
	if err != nil {
		t.Fatal(err)
	}
	if !solana.IsOnCurve(deployerBytes) {
		t.Fatal("a real ed25519 public key must be on-curve")
	}

	pda, err := solana.GetMerkleAccountPublicKey(programID, originalDeployer)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base58.Decode(pda)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 32 {
		t.Fatalf("pda length = %d, want 32", len(raw))
	}
	if solana.IsOnCurve(raw) {
		t.Fatal("a program derived address must be off-curve")
	}
}
