package privatewallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type emporiumMetadataTuple struct {
	V             uint8
	R             [32]byte
	S             [32]byte
	SignerAddress common.Address
	Ops           []emporiumOperation
	MaxFee        *big.Int
	Deadline      *big.Int
}

const (
	emporiumSignatureDefaultTTL = 20 * time.Minute
	emporiumMaxExecuteAfter     = 2 * 24 * time.Hour
)

var emporiumOpComponents = []abi.ArgumentMarshaling{
	{Name: "endpoint", Type: "address"},
	{Name: "invokeWallet", Type: "bool"},
	{Name: "value", Type: "uint128"},
	{Name: "callData", Type: "bytes"},
}

func EncodeEmporiumMetadata(
	chainID int,
	emporiumAddress string,
	privateKey string,
	ops []string,
	emporiumMessage *big.Int,
	signerAddress string,
	maxFee *big.Int,
	executeAfter *int,
) (string, error) {
	if emporiumMessage == nil {
		emporiumMessage = big.NewInt(0)
	}

	var v uint8
	var r, s [32]byte
	deadline := big.NewInt(0)
	if privateKey != "" && signerAddress != "" {
		if maxFee == nil {
			return "", errors.New("privatewallet: maxFee required if making tx from emporium")
		}
		// Scheduled txs execute at executeAfter (unix seconds), so the signature must outlive that time.
		now := time.Now()
		signedFrom := now
		if executeAfter != nil {
			executeAt := time.Unix(int64(*executeAfter), 0)
			if executeAt.Sub(now) > emporiumMaxExecuteAfter {
				return "", errors.New("privatewallet: executeAfter must be a unix timestamp in seconds at most 2 days ahead")
			}
			if executeAt.After(now) {
				signedFrom = executeAt
			}
		}
		deadline = big.NewInt(signedFrom.Add(emporiumSignatureDefaultTTL).Unix())
		var err error
		v, r, s, err = signEmporium(chainID, emporiumAddress, privateKey, ops, emporiumMessage, maxFee, deadline)
		if err != nil {
			return "", err
		}
	}
	if signerAddress == "" {
		signerAddress = constants.ZeroAddress
	}
	if maxFee == nil {
		maxFee = big.NewInt(0)
	}

	opStructs := make([]emporiumOperation, len(ops))
	for i, op := range ops {
		operation, err := decodeEmporiumOp(op)
		if err != nil {
			return "", err
		}
		opStructs[i] = operation
	}

	tupleType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "v", Type: "uint8"},
		{Name: "r", Type: "bytes32"},
		{Name: "s", Type: "bytes32"},
		{Name: "signerAddress", Type: "address"},
		{Name: "ops", Type: "tuple[]", Components: emporiumOpComponents},
		{Name: "maxFee", Type: "uint256"},
		{Name: "deadline", Type: "uint256"},
	})
	if err != nil {
		return "", fmt.Errorf("emporium metadata tuple type: %w", err)
	}

	packed, err := (abi.Arguments{{Type: tupleType}}).Pack(emporiumMetadataTuple{
		V:             v,
		R:             r,
		S:             s,
		SignerAddress: common.HexToAddress(signerAddress),
		Ops:           opStructs,
		MaxFee:        maxFee,
		Deadline:      deadline,
	})
	if err != nil {
		return "", fmt.Errorf("emporium metadata pack: %w", err)
	}

	return "0x" + hex.EncodeToString(packed), nil
}

func signEmporium(
	chainID int,
	emporiumAddress string,
	privateKey string,
	ops []string,
	emporiumMessage *big.Int,
	maxFee *big.Int,
	deadline *big.Int,
) (uint8, [32]byte, [32]byte, error) {
	var r, s [32]byte
	if constants.IsSolanaLike(chainID) {
		return 0, r, s, errors.New("privatewallet: solana does not support typed data")
	}

	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return 0, r, s, fmt.Errorf("emporium signer key: %w", err)
	}

	opsValues := make([]interface{}, len(ops))
	for i, op := range ops {
		operation, err := decodeEmporiumOp(op)
		if err != nil {
			return 0, r, s, err
		}
		opsValues[i] = apitypes.TypedDataMessage{
			"endpoint":     operation.Endpoint.Hex(),
			"invokeWallet": operation.InvokeWallet,
			"value":        operation.Value.String(),
			"callData":     operation.CallData,
		}
	}

	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"EmporiumSignature": {
				{Name: "message", Type: "uint256"},
				{Name: "ops", Type: "EmporiumOperation[]"},
				{Name: "maxFee", Type: "uint256"},
				{Name: "deadline", Type: "uint256"},
			},
			"EmporiumOperation": {
				{Name: "endpoint", Type: "address"},
				{Name: "invokeWallet", Type: "bool"},
				{Name: "value", Type: "uint128"},
				{Name: "callData", Type: "bytes"},
			},
		},
		PrimaryType: "EmporiumSignature",
		Domain: apitypes.TypedDataDomain{
			Name:              "Emporium",
			Version:           "1.0.0",
			ChainId:           math.NewHexOrDecimal256(int64(chainID)),
			VerifyingContract: emporiumAddress,
		},
		Message: apitypes.TypedDataMessage{
			"message":  emporiumMessage.String(),
			"ops":      opsValues,
			"maxFee":   maxFee.String(),
			"deadline": deadline.String(),
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return 0, r, s, err
	}
	structHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return 0, r, s, err
	}

	raw := append([]byte{0x19, 0x01}, domainSeparator...)
	raw = append(raw, structHash...)
	digest := gethcrypto.Keccak256(raw)

	sig, err := gethcrypto.Sign(digest, key)
	if err != nil {
		return 0, r, s, err
	}
	copy(r[:], sig[0:32])
	copy(s[:], sig[32:64])
	return sig[64] + 27, r, s, nil
}

func SignerAddressFromPrivateKey(chainID int, privateKey string) (string, error) {
	if constants.IsSolanaLike(chainID) {
		return SolanaSignerAddressFromPrivateKey(privateKey)
	}
	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return "", fmt.Errorf("signer key: %w", err)
	}
	return gethcrypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}
