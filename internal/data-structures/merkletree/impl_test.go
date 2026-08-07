package merkletree_test

import (
	"math/big"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
)

func newTestTree() merkletree.MerkleTree {
	return merkletree.New(crypto.PoseidonHashFunc, big.NewInt(0), 4)
}

func fillTree(t *testing.T, tree merkletree.MerkleTree, values ...int64) []*big.Int {
	t.Helper()
	start := tree.GetStartIndex()
	leaves := make([]*big.Int, len(values))
	for i, v := range values {
		leaves[i] = big.NewInt(v)
		idx := new(big.Int).Add(start, big.NewInt(int64(i)))
		tree.Insert(leaves[i], idx)
	}
	return leaves
}

func TestMerkleTree_RootRequiresNoGapsInTheInsertedRange(t *testing.T) {
	tree := newTestTree()
	start := tree.GetStartIndex()

	tree.Insert(big.NewInt(1), start)
	if _, err := tree.GetRootHash(); err != nil {
		t.Fatalf("a single contiguous insert should already be complete: %v", err)
	}

	tree.Insert(big.NewInt(2), new(big.Int).Add(start, big.NewInt(2)))
	if _, err := tree.GetRootHash(); err == nil {
		t.Fatal("expected an error: slot start+1 was never inserted")
	}
}

func TestMerkleTree_SiblingProofReconstructsTheRoot(t *testing.T) {
	tree := newTestTree()
	leaves := fillTree(t, tree, 101, 102, 103, 104, 105, 106, 107, 108)

	root, err := tree.GetRootHash()
	if err != nil {
		t.Fatalf("GetRootHash: %v", err)
	}

	for i, leaf := range leaves {
		siblings, err := tree.GetSiblingHashesForVerification(leaf)
		if err != nil {
			t.Fatalf("leaf %d: GetSiblingHashesForVerification: %v", i, err)
		}
		sides, err := tree.GetSiblingSides(leaf)
		if err != nil {
			t.Fatalf("leaf %d: GetSiblingSides: %v", i, err)
		}

		cur := new(big.Int).Set(leaf)
		for level := 0; level < 3; level++ {
			if sides[level].Sign() == 0 { // leaf is the left child
				cur = crypto.PoseidonHashFunc(cur, siblings[level])
			} else {
				cur = crypto.PoseidonHashFunc(siblings[level], cur)
			}
		}
		if cur.Cmp(root) != 0 {
			t.Fatalf("leaf %d (%s): reconstructed root %s, want %s", i, leaf, cur, root)
		}
	}
}

func TestMerkleTree_UnknownLeafGetsZeroSiblings(t *testing.T) {
	tree := newTestTree()
	fillTree(t, tree, 1, 2, 3, 4, 5, 6, 7, 8)

	siblings, err := tree.GetSiblingHashesForVerification(big.NewInt(999))
	if err != nil {
		t.Fatalf("GetSiblingHashesForVerification: %v", err)
	}
	for _, s := range siblings {
		if s.Sign() != 0 {
			t.Fatalf("expected all-zero siblings for an unknown leaf, got %s", s)
		}
	}
}

func TestMerkleTree_LastLeavesReturnsMostRecentFirst(t *testing.T) {
	tree := newTestTree()
	fillTree(t, tree, 1, 2, 3)

	got := tree.LastLeaves(2)
	if len(got) != 2 || got[0].Int64() != 3 || got[1].Int64() != 2 {
		t.Fatalf("LastLeaves(2) = %v, want [3 2]", got)
	}
}

func TestMerkleTree_CloneIsIndependent(t *testing.T) {
	tree := newTestTree()
	fillTree(t, tree, 1, 2, 3, 4, 5, 6, 7, 8)
	rootBefore, err := tree.GetRootHash()
	if err != nil {
		t.Fatalf("GetRootHash: %v", err)
	}

	clone := tree.Clone()
	clone.Remove(tree.GetStartIndex())

	rootAfter, err := tree.GetRootHash()
	if err != nil {
		t.Fatalf("GetRootHash after clone mutation: %v", err)
	}
	if rootAfter.Cmp(rootBefore) != 0 {
		t.Fatal("mutating the clone changed the original tree's root")
	}
}

func TestMerkleTree_ToJSONFromJSONRoundTrip(t *testing.T) {
	tree := merkletree.New(crypto.PoseidonHashFunc, big.NewInt(0), 0)
	fillTree(t, tree, 1, 2, 3, 4, 5, 6, 7, 8)
	wantRoot, err := tree.GetRootHash()
	if err != nil {
		t.Fatalf("GetRootHash: %v", err)
	}

	restored, err := merkletree.FromJSON(tree.ToJSON(), crypto.PoseidonHashFunc, big.NewInt(0))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	gotRoot, err := restored.GetRootHash()
	if err != nil {
		t.Fatalf("GetRootHash on restored tree: %v", err)
	}
	if gotRoot.Cmp(wantRoot) != 0 {
		t.Fatalf("restored root = %s, want %s", gotRoot, wantRoot)
	}
	if v, ok := restored.GetValue(tree.GetStartIndex()); !ok || v.Int64() != 1 {
		t.Fatalf("restored leaf = %v (ok=%v), want 1", v, ok)
	}
}
