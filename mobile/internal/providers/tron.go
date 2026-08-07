package providers

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type HostTronSigner interface {
	Address() (string, error)
	SignMessage(message string) ([]byte, error)
	SignTxHash(txHash []byte) ([]byte, error)
}

type TronHostSigner struct {
	host HostTronSigner
}

var _ types.TronSigner = (*TronHostSigner)(nil)

func NewTronHostSigner(host HostTronSigner) *TronHostSigner {
	return &TronHostSigner{host: host}
}

func (s *TronHostSigner) GetAddress(_ context.Context) (string, error) {
	return s.host.Address()
}

func (s *TronHostSigner) SignMessage(_ context.Context, message string) ([]byte, error) {
	return s.host.SignMessage(message)
}

func (s *TronHostSigner) SignTxHash(_ context.Context, txHash []byte) ([]byte, error) {
	return s.host.SignTxHash(txHash)
}
