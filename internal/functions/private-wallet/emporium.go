package privatewallet

import (
	"bytes"
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
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
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

// EncodeEmporiumMetadata leaves v, r, s empty: the wallet signs after prepare (SignEmporiumTx)
// and the signature is inserted with SetEmporiumSignatureWords.
func EncodeEmporiumMetadata(
	ops []string,
	signerAddress string,
	maxFee *big.Int,
	executeAfter *int,
) (string, error) {
	deadline := big.NewInt(0)
	if signerAddress != "" {
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

// SignEmporiumTx is the wallet's EIP-712 signature over a job's signedMessageHash.
func SignEmporiumTx(
	chainID int,
	emporiumAddress string,
	privateKey string,
	signedMessageHash *big.Int,
) (types.EmporiumSignature, error) {
	if constants.IsSolanaLike(chainID) {
		return types.EmporiumSignature{}, errors.New("privatewallet: solana does not support typed data")
	}

	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return types.EmporiumSignature{}, fmt.Errorf("emporium signer key: %w", err)
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
				{Name: "signedMessageHash", Type: "uint256"},
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
			"signedMessageHash": signedMessageHash.String(),
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return types.EmporiumSignature{}, err
	}
	structHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return types.EmporiumSignature{}, err
	}

	raw := append([]byte{0x19, 0x01}, domainSeparator...)
	raw = append(raw, structHash...)
	digest := gethcrypto.Keccak256(raw)

	sig, err := gethcrypto.Sign(digest, key)
	if err != nil {
		return types.EmporiumSignature{}, err
	}
	return types.EmporiumSignature{
		V: sig[64] + 27,
		R: "0x" + hex.EncodeToString(sig[0:32]),
		S: "0x" + hex.EncodeToString(sig[32:64]),
	}, nil
}

// v, r, s are the first three head words of abi.encode(EmporiumStack): bytes [32, 128).
// The contract skips them in calldataHash (CircomDataBuilder.clearEmporiumSignature).
const (
	emporiumSignatureStart = 32
	emporiumSignatureEnd   = 128
)

var errEmporiumSignatureSlotUnavailable = errors.New("privatewallet: emporium metadata has no empty signature slot")

// SetEmporiumSignatureWords returns metadata with the empty v, r, s words replaced by sig.
func SetEmporiumSignatureWords(metadata string, sig types.EmporiumSignature) (string, error) {
	raw := common.FromHex(metadata)
	if len(raw) < emporiumSignatureEnd {
		return "", errEmporiumSignatureSlotUnavailable
	}
	slot := raw[emporiumSignatureStart:emporiumSignatureEnd]
	if !bytes.Equal(slot, make([]byte, len(slot))) {
		return "", errEmporiumSignatureSlotUnavailable
	}
	r, s := common.FromHex(sig.R), common.FromHex(sig.S)
	if len(r) != 32 || len(s) != 32 {
		return "", errors.New("privatewallet: invalid emporium signature")
	}
	slot[31] = sig.V
	copy(slot[32:64], r)
	copy(slot[64:96], s)
	return "0x" + hex.EncodeToString(raw), nil
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
