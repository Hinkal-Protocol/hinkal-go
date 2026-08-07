package mobile

import (
	"context"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	hinkal "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/hinkal"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type ProviderAdapter struct {
	a types.IProviderAdapter
}

func (p *ProviderAdapter) GetChainID() int64 {
	if cid := p.a.GetChainID(); cid != nil {
		return int64(*cid)
	}
	return 0
}

func (h *Hinkal) GetProviderAdapter(chainID int64) (*ProviderAdapter, error) {
	cid := int(chainID)
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	adapter, err := h.c.h.GetProviderAdapter(&cid)
	if err != nil {
		return nil, err
	}
	return &ProviderAdapter{a: adapter}, nil
}

func (h *Hinkal) InitProviderAdapter(adapter *ProviderAdapter) error {
	if adapter == nil || adapter.a == nil {
		return mobileerrors.ErrNilProviderAdapter
	}
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return h.c.h.InitProviderAdapter(context.Background(), adapter.a)
}

func (h *Hinkal) ResetProviderAdapters() {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	hinkal.ResetProviderAdapters(h.c.h)
}

func (h *Hinkal) Destroy() error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.Destroy(h.c.h)
}

func (h *Hinkal) DisconnectFromConnector() error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.DisconnectFromConnector(h.c.h)
}

func (h *Hinkal) InitUserKeys(mode string) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.InitUserKeys(h.c.h, mode)
}

func (h *Hinkal) StoreAndGetInitialSignature(authSignature string, isSolanaLedger bool, txMessageForSolanaLedger string) (string, error) {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.StoreAndGetInitialSignature(h.c.h, authSignature, isSolanaLedger, txMessageForSolanaLedger)
}

func (h *Hinkal) SignMessage(message string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.SignMessage(h.c.h, message)
}

func (h *Hinkal) SignTypedData(typedDataHash []byte) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.SignTypedData(h.c.h, typedDataHash)
}

// The SDK takes the wait deadline via ctx; gomobile cannot pass a context, so timeoutSec (<=0 means no timeout) stands in for it.
func (h *Hinkal) WaitForTransaction(chainID int64, txHash string, confirmations, timeoutSec int64) (bool, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.WaitForTransaction(h.c.h, chainID, txHash, confirmations, timeoutSec)
}

func (h *Hinkal) MonitorConnectedAddress(chainID int64) error {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.MonitorConnectedAddress(h.c.h, chainID)
}

func (h *Hinkal) SwitchNetwork(networkJSON string) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.SwitchNetwork(h.c.h, networkJSON)
}

func (h *Hinkal) SwitchNetworkByChainID(chainID int64) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.SwitchNetworkByChainID(h.c.h, chainID)
}

func (h *Hinkal) IsSelectedNetworkSupported(chainID int64) bool {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.IsSelectedNetworkSupported(h.c.h, chainID)
}

func (h *Hinkal) GetEthereumAddress() (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetEthereumAddress(h.c.h)
}

func (h *Hinkal) GetEthereumAddressByChain(chainID int64) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetEthereumAddressByChain(h.c.h, chainID)
}

func (h *Hinkal) GetSolanaPublicKey() (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetSolanaPublicKey(h.c.h)
}

func (h *Hinkal) InitUserKeysWithEnclaveSignature(mode string) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.InitUserKeysWithEnclaveSignature(h.c.h, mode)
}
