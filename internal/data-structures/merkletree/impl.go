package merkletree

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const MerkleLevels uint = 200

type merkleTreeImpl struct {
	hashFn       HashFunc
	levels       uint
	defaultValue *big.Int
	tree         map[string]*big.Int
	reverseTree  map[string]*big.Int
	count        *big.Int
	index        *big.Int
}

func New(hash HashFunc, defaultValue *big.Int, levels uint) MerkleTree {
	if levels == 0 {
		levels = MerkleLevels
	}
	si := startIdx(levels)
	return &merkleTreeImpl{
		hashFn:       hash,
		levels:       levels,
		defaultValue: new(big.Int).Set(defaultValue),
		tree:         make(map[string]*big.Int),
		reverseTree:  make(map[string]*big.Int),
		count:        new(big.Int),
		index:        new(big.Int).Set(si),
	}
}

func FromJSON(j types.MerkleTreeJSON, hash HashFunc, defaultValue *big.Int) (MerkleTree, error) {
	tree, err := parseStringMap(j.Tree)
	if err != nil {
		return nil, fmt.Errorf("parse tree: %w", err)
	}

	var reverseTree map[string]*big.Int
	if j.ReverseTree != nil {
		reverseTree, err = parseStringMap(j.ReverseTree)
		if err != nil {
			return nil, fmt.Errorf("parse reverseTree: %w", err)
		}
	}

	count := new(big.Int)
	if _, ok := count.SetString(j.Count, 10); !ok {
		return nil, fmt.Errorf("invalid count: %s", j.Count)
	}

	index := new(big.Int)
	if _, ok := index.SetString(j.Index, 10); !ok {
		return nil, fmt.Errorf("invalid index: %s", j.Index)
	}

	m := &merkleTreeImpl{
		hashFn:       hash,
		levels:       MerkleLevels,
		defaultValue: new(big.Int).Set(defaultValue),
		tree:         tree,
		count:        count,
		index:        index,
	}

	if reverseTree != nil {
		m.reverseTree = reverseTree
	} else {
		m.reverseTree = m.buildReverseTree()
	}

	return m, nil
}

func (m *merkleTreeImpl) GetStartIndex() *big.Int {
	return startIdx(m.levels)
}

func (m *merkleTreeImpl) GetIndex() *big.Int {
	return new(big.Int).Set(m.index)
}

func (m *merkleTreeImpl) Insert(value, nodeIndex *big.Int) {
	if _, exists := m.tree[nodeIndex.String()]; !exists {
		m.count.Add(m.count, big.NewInt(1))
	}
	next := new(big.Int).Add(nodeIndex, big.NewInt(1))
	if next.Cmp(m.index) > 0 {
		m.index.Set(next)
	}
	m.forceInsert(value, nodeIndex)
}

func (m *merkleTreeImpl) Remove(nodeIndex *big.Int) {
	m.forceInsert(new(big.Int).Set(m.defaultValue), nodeIndex)
}

func (m *merkleTreeImpl) GetValue(index *big.Int) (*big.Int, bool) {
	v, ok := m.tree[index.String()]
	return v, ok
}

func (m *merkleTreeImpl) LastLeaves(limit int) []*big.Int {
	if m.count.Sign() == 0 {
		return nil
	}
	startIndex := m.GetStartIndex()
	out := make([]*big.Int, 0, limit)
	for i := 0; i < limit && big.NewInt(int64(i)).Cmp(m.count) < 0; i++ {
		index := new(big.Int).Add(startIndex, m.count)
		index.Sub(index, big.NewInt(1+int64(i)))
		value, ok := m.GetValue(index)
		if ok && value != nil && value.Sign() != 0 {
			out = append(out, value)
		}
	}
	return out
}

func (m *merkleTreeImpl) GetRootHash() (*big.Int, error) {
	si := m.GetStartIndex()
	expected := new(big.Int).Sub(m.index, si)
	if m.count.Cmp(expected) != 0 {
		return nil, errors.New("merkle tree is incomplete")
	}
	limit := new(big.Int).Lsh(big.NewInt(1), m.levels)
	for i := big.NewInt(1); i.Cmp(limit) < 0; i.Lsh(i, 1) {
		if v := m.tree[i.String()]; v != nil && v.Sign() != 0 {
			return new(big.Int).Set(v), nil
		}
	}
	return new(big.Int).Set(m.defaultValue), nil
}

const CircomMerkleLength = 25

func (m *merkleTreeImpl) completenessCheck() error {
	si := m.GetStartIndex()
	expected := new(big.Int).Sub(m.index, si)
	if m.count.Cmp(expected) != 0 {
		return errors.New("merkle tree is incomplete")
	}
	return nil
}

func (m *merkleTreeImpl) getSiblingIndex(index *big.Int) *big.Int {
	if index.Cmp(big.NewInt(1)) == 0 {
		return big.NewInt(1)
	}
	if index.Bit(0) == 1 { // odd
		return new(big.Int).Sub(index, big.NewInt(1))
	}
	return new(big.Int).Add(index, big.NewInt(1))
}

func zeroSlice(n int) []*big.Int {
	out := make([]*big.Int, n)
	for i := range out {
		out[i] = big.NewInt(0)
	}
	return out
}

func sliceFirst(values []*big.Int, n int) []*big.Int {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

// get sibling hashes needed by main.circom
func (m *merkleTreeImpl) GetSiblingHashesForVerification(item *big.Int) ([]*big.Int, error) {
	if err := m.completenessCheck(); err != nil {
		return nil, err
	}
	idx, ok := m.reverseTree[item.String()]
	if !ok {
		return zeroSlice(CircomMerkleLength), nil
	}
	index := new(big.Int).Set(idx)
	hashes := make([]*big.Int, 0)
	zero := big.NewInt(0)
	for index.Cmp(zero) != 0 {
		sib := m.getSiblingIndex(index)
		v, ok := m.tree[sib.String()]
		if !ok || v == nil {
			v = m.defaultValue
		}
		hashes = append(hashes, new(big.Int).Set(v))
		index.Rsh(index, 1)
	}
	return sliceFirst(hashes, CircomMerkleLength), nil
}

// get item's sibling hashes side
func (m *merkleTreeImpl) GetSiblingSides(item *big.Int) ([]*big.Int, error) {
	if err := m.completenessCheck(); err != nil {
		return nil, err
	}
	idx, ok := m.reverseTree[item.String()]
	if !ok {
		return zeroSlice(CircomMerkleLength), nil
	}
	index := new(big.Int).Set(idx)
	sides := make([]*big.Int, 0)
	zero := big.NewInt(0)
	for index.Cmp(zero) != 0 {
		if index.Bit(0) == 0 { // left = 0, right = 1
			sides = append(sides, big.NewInt(0))
		} else {
			sides = append(sides, big.NewInt(1))
		}
		index.Rsh(index, 1)
	}
	return sliceFirst(sides, CircomMerkleLength), nil
}

func (m *merkleTreeImpl) ToJSON() types.MerkleTreeJSON {
	tree := make(map[string]string, len(m.tree))
	for k, v := range m.tree {
		tree[k] = v.String()
	}
	reverseTree := make(map[string]string, len(m.reverseTree))
	for k, v := range m.reverseTree {
		reverseTree[k] = v.String()
	}
	return types.MerkleTreeJSON{
		Tree:        tree,
		ReverseTree: reverseTree,
		Count:       m.count.String(),
		Index:       m.index.String(),
	}
}

func (m *merkleTreeImpl) ToJSONLite() types.MerkleTreeJSON {
	tree := make(map[string]string, len(m.tree))
	for k, v := range m.tree {
		tree[k] = v.String()
	}
	return types.MerkleTreeJSON{
		Tree:  tree,
		Count: m.count.String(),
		Index: m.index.String(),
	}
}

func (m *merkleTreeImpl) Clone() MerkleTree {
	tree := make(map[string]*big.Int, len(m.tree))
	for k, v := range m.tree {
		tree[k] = new(big.Int).Set(v)
	}
	reverseTree := make(map[string]*big.Int, len(m.reverseTree))
	for k, v := range m.reverseTree {
		reverseTree[k] = new(big.Int).Set(v)
	}
	return &merkleTreeImpl{
		hashFn:       m.hashFn,
		levels:       m.levels,
		defaultValue: new(big.Int).Set(m.defaultValue),
		tree:         tree,
		reverseTree:  reverseTree,
		count:        new(big.Int).Set(m.count),
		index:        new(big.Int).Set(m.index),
	}
}

func (m *merkleTreeImpl) forceInsert(value, nodeIndex *big.Int) {
	si := m.GetStartIndex()
	if nodeIndex.Cmp(si) < 0 {
		panic(fmt.Sprintf("nodeIndex %s below startIndex %s", nodeIndex, si))
	}

	m.tree[nodeIndex.String()] = value
	m.reverseTree[value.String()] = new(big.Int).Set(nodeIndex)

	fullCount := new(big.Int).Sub(m.index, si)
	twoPower := logarithm2(fullCount)

	cur := new(big.Int).Set(nodeIndex)
	for i := uint(1); i <= twoPower; i++ {
		cur.Rsh(cur, 1)
		left := new(big.Int).Lsh(cur, 1)
		right := new(big.Int).Add(left, big.NewInt(1))

		leftVal := m.tree[left.String()]
		if leftVal == nil {
			leftVal = m.defaultValue
		}
		rightVal := m.tree[right.String()]
		if rightVal == nil {
			rightVal = m.defaultValue
		}
		m.tree[cur.String()] = m.hashFn(leftVal, rightVal)
	}
}

func (m *merkleTreeImpl) buildReverseTree() map[string]*big.Int {
	rt := make(map[string]*big.Int)
	si := m.GetStartIndex()
	end := new(big.Int).Add(si, m.count)
	for i := new(big.Int).Set(si); i.Cmp(end) < 0; i.Add(i, big.NewInt(1)) {
		if v, ok := m.tree[i.String()]; ok {
			rt[v.String()] = new(big.Int).Set(i)
		}
	}
	return rt
}

func startIdx(levels uint) *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), levels-1)
}

func logarithm2(n *big.Int) uint {
	if n.Sign() <= 0 {
		return 0
	}
	return uint(new(big.Int).Sub(n, big.NewInt(1)).BitLen())
}

func parseStringMap(m map[string]string) (map[string]*big.Int, error) {
	out := make(map[string]*big.Int, len(m))
	for k, v := range m {
		n := new(big.Int)
		if _, ok := n.SetString(v, 10); !ok {
			return nil, fmt.Errorf("cannot parse %q as big.Int", v)
		}
		out[k] = n
	}
	return out, nil
}
