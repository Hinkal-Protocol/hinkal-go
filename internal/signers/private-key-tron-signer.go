package signers

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type PrivateKeyTronSigner struct {
	key     *ecdsa.PrivateKey
	address string
}

func NewPrivateKeyTronSigner(privateKeyHex string) (*PrivateKeyTronSigner, error) {
	hex := strings.TrimPrefix(privateKeyHex, "0x")
	key, err := crypto.HexToECDSA(hex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	address, err := utils.EVMHexToTronBase58Address(crypto.PubkeyToAddress(key.PublicKey).Hex())
	if err != nil {
		return nil, fmt.Errorf("derive tron address: %w", err)
	}
	return &PrivateKeyTronSigner{key: key, address: address}, nil
}

func (s *PrivateKeyTronSigner) GetAddress(_ context.Context) (string, error) {
	return s.address, nil
}

func (s *PrivateKeyTronSigner) SignMessage(_ context.Context, message string) ([]byte, error) {
	data := []byte(message)
	msg := fmt.Sprintf("\x19TRON Signed Message:\n%d%s", len(data), message)
	hash := crypto.Keccak256([]byte(msg))
	sig, err := crypto.Sign(hash, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	// Tron personal_sign uses same 27/28 convention as Ethereum for message signatures.
	sig[64] += 27
	return sig, nil
}

func (s *PrivateKeyTronSigner) SignTxHash(_ context.Context, txHash []byte) ([]byte, error) {
	sig, err := crypto.Sign(txHash, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	return sig, nil
}
