package providers

import (
	"context"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type HostSolanaSigner interface {
	PublicKey() (string, error)
	SignMessage(message []byte) ([]byte, error)
	SignTransaction(tx []byte) ([]byte, error)
}

type SolanaHostSigner struct {
	host HostSolanaSigner
}

var _ types.SolanaSigner = (*SolanaHostSigner)(nil)

func NewSolanaHostSigner(host HostSolanaSigner) *SolanaHostSigner {
	return &SolanaHostSigner{host: host}
}

func (s *SolanaHostSigner) GetPublicKey(_ context.Context) (solana.PublicKey, error) {
	pub, err := s.host.PublicKey()
	if err != nil {
		return solana.PublicKey{}, err
	}
	return solana.PublicKeyFromBase58(pub)
}

func (s *SolanaHostSigner) SignMessage(_ context.Context, message []byte) ([]byte, error) {
	return s.host.SignMessage(message)
}

func (s *SolanaHostSigner) SignTransaction(_ context.Context, tx *solana.Transaction) (*solana.Transaction, error) {
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	signed, err := s.host.SignTransaction(raw)
	if err != nil {
		return nil, err
	}
	return solana.TransactionFromDecoder(bin.NewBinDecoder(signed))
}
