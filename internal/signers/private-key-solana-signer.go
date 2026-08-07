package signers

import (
	"context"
	"fmt"

	solana "github.com/gagliardetto/solana-go"
)

type PrivateKeySolanaSigner struct {
	key    solana.PrivateKey
	pubKey solana.PublicKey
}

func NewPrivateKeySolanaSigner(privateKeyBase58 string) (*PrivateKeySolanaSigner, error) {
	key, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return &PrivateKeySolanaSigner{
		key:    key,
		pubKey: key.PublicKey(),
	}, nil
}

func (s *PrivateKeySolanaSigner) GetPublicKey(_ context.Context) (solana.PublicKey, error) {
	return s.pubKey, nil
}

func (s *PrivateKeySolanaSigner) SignMessage(_ context.Context, message []byte) ([]byte, error) {
	sig, err := s.key.Sign(message)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	return sig[:], nil
}

func (s *PrivateKeySolanaSigner) SignTransaction(_ context.Context, tx *solana.Transaction) (*solana.Transaction, error) {
	_, err := tx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(s.pubKey) {
			return &s.key
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	return tx, nil
}
