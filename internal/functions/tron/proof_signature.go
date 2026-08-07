package tron

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func mustABIType(t string) abi.Type {
	ty, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(fmt.Sprintf("tron: invalid abi type %q: %v", t, err))
	}
	return ty
}

var (
	abiUint16  = mustABIType("uint16")
	abiUint256 = mustABIType("uint256")
)

// ParseZkCalldata splits the zkCallData tuple and reorders the G2 b coordinates for the Tron flow.
func ParseZkCalldata(zk types.NewZkCallDataType) (a []*big.Int, b [][]*big.Int, c, publicSignals []*big.Int, err error) {
	parse := func(values []string) ([]*big.Int, error) {
		out := make([]*big.Int, len(values))
		for i, v := range values {
			n, e := utils.ParseBigInt(v)
			if e != nil {
				return nil, e
			}
			out[i] = n
		}
		return out, nil
	}
	if a, err = parse(zk.A[:]); err != nil {
		return
	}
	b0, err0 := parse(zk.B[0][:])
	if err0 != nil {
		return nil, nil, nil, nil, err0
	}
	b1, err1 := parse(zk.B[1][:])
	if err1 != nil {
		return nil, nil, nil, nil, err1
	}
	if c, err = parse(zk.C[:]); err != nil {
		return
	}
	if publicSignals, err = parse(zk.PublicSignals); err != nil {
		return
	}
	b = [][]*big.Int{
		{b0[1], b0[0]},
		{b1[1], b1[0]},
	}
	return a, b, c, publicSignals, nil
}

// GetVerifierID hashes the circuit dimensions and external action id into the verifier id.
func GetVerifierID(dim types.DimDataType, externalActionID *big.Int) (*big.Int, error) {
	packed, err := (abi.Arguments{
		{Type: abiUint16}, {Type: abiUint16}, {Type: abiUint16}, {Type: abiUint256},
	}).Pack(uint16(dim.TokenNumber), uint16(dim.NullifierAmount), uint16(dim.OutputAmount), externalActionID)
	if err != nil {
		return nil, fmt.Errorf("tron: verifier id encode: %w", err)
	}
	return new(big.Int).SetBytes(gethcrypto.Keccak256(packed)), nil
}

func bigIntsToDecimal(values []*big.Int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

// GenerateProofSignatureRemotely requests the Tron proof signature from the enclave /sign-proof route.
func GenerateProofSignatureRemotely(
	ctx context.Context,
	chainID int,
	a []*big.Int,
	b [][]*big.Int,
	c []*big.Int,
	inputs []*big.Int,
	dim types.DimDataType,
	externalActionID *big.Int,
) (api.TronProofSignature, error) {
	verifierID, err := GetVerifierID(dim, externalActionID)
	if err != nil {
		return api.TronProofSignature{}, err
	}
	signatureHex, err := api.SignProofEnclaveCall(ctx, api.SignProofRequest{
		ChainID:         chainID,
		A:               bigIntsToDecimal(a),
		B:               [][]string{bigIntsToDecimal(b[0]), bigIntsToDecimal(b[1])},
		C:               bigIntsToDecimal(c),
		Inputs:          bigIntsToDecimal(inputs),
		VerifierID:      verifierID.String(),
		TokenNumber:     dim.TokenNumber,
		NullifierAmount: dim.NullifierAmount,
		OutputAmount:    dim.OutputAmount,
	})
	if err != nil {
		return api.TronProofSignature{}, err
	}
	return splitSignature(signatureHex)
}

func splitSignature(signatureHex string) (api.TronProofSignature, error) {
	sig := common.FromHex(strings.TrimSpace(signatureHex))
	if len(sig) != 65 {
		return api.TronProofSignature{}, fmt.Errorf("tron: unexpected signature length %d", len(sig))
	}
	v := int(sig[64])
	if v < 27 {
		v += 27
	}
	return api.TronProofSignature{
		V: v,
		R: "0x" + common.Bytes2Hex(sig[0:32]),
		S: "0x" + common.Bytes2Hex(sig[32:64]),
	}, nil
}

func SwapTronBCoordinate(zk *types.NewZkCallDataType) {
	zk.B = [2][2]string{
		{zk.B[0][1], zk.B[0][0]},
		{zk.B[1][1], zk.B[1][0]},
	}
}

// ReorderZkCallData computes the Tron proof signature and, when modify is set, swaps the G2 b
// coordinates of zkCallData in place.
func ReorderZkCallData(
	ctx context.Context,
	chainID int,
	zk *types.NewZkCallDataType,
	dim types.DimDataType,
	circom types.CircomDataType,
	modify bool,
) (api.TronProofSignature, error) {
	a, b, c, publicSignals, err := ParseZkCalldata(*zk)
	if err != nil {
		return api.TronProofSignature{}, err
	}
	externalActionID := circom.ExternalActionData.ExternalActionID
	if externalActionID == nil {
		externalActionID = big.NewInt(0)
	}
	proofSig, err := GenerateProofSignatureRemotely(ctx, chainID, a, b, c, publicSignals, dim, externalActionID)
	if err != nil {
		return api.TronProofSignature{}, err
	}
	if modify {
		SwapTronBCoordinate(zk)
	}
	return proofSig, nil
}
