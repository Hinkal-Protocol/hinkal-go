package crypto

import (
	"math/big"

	"github.com/iden3/go-iden3-crypto/babyjub"
	iden3utils "github.com/iden3/go-iden3-crypto/utils"
)

func eddsaPruneBuffer(buf *[32]byte) {
	buf[0] &= 0xF8
	buf[31] &= 0x7F
	buf[31] |= 0x40
}

func eddsaScalar(prv []byte) *big.Int {
	h := babyjub.Blake512(prv)
	var b [32]byte
	copy(b[:], h[:32])
	eddsaPruneBuffer(&b)
	return iden3utils.SetBigIntFromLEBytes(new(big.Int), b[:])
}

func EddsaPrv2Pub(prv []byte) *babyjub.Point {
	s := new(big.Int).Rsh(eddsaScalar(prv), 3)
	return babyjub.NewPoint().Mul(s, babyjub.B8)
}

func EddsaSignPoseidon(prv []byte, msg *big.Int) (*babyjub.Point, *big.Int, error) {
	a := EddsaPrv2Pub(prv)
	s := new(big.Int).Rsh(eddsaScalar(prv), 3)

	h1 := babyjub.Blake512(prv)
	msgLE := iden3utils.BigIntLEBytes(msg)
	rBuf := babyjub.Blake512(append(append([]byte{}, h1[32:]...), msgLE[:]...))
	r := iden3utils.SetBigIntFromLEBytes(new(big.Int), rBuf)
	r.Mod(r, babyjub.SubOrder)

	r8 := babyjub.NewPoint().Mul(r, babyjub.B8)

	hm, err := PoseidonBig(r8.X, r8.Y, a.X, a.Y, msg)
	if err != nil {
		return nil, nil, err
	}

	sig := new(big.Int).Mul(hm, new(big.Int).Lsh(s, 3))
	sig.Add(r, sig)
	sig.Mod(sig, babyjub.SubOrder)
	return r8, sig, nil
}
