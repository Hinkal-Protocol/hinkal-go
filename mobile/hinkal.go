package mobile

import (
	hinkal "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/hinkal"
)

type Hinkal struct {
	c *Client
}

func (c *Client) Hinkal() *Hinkal {
	return &Hinkal{c: c}
}

func (h *Hinkal) InitUserKeysWithSignature(signature string) {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	hinkal.InitUserKeysWithSignature(h.c.h, signature)
}

func (h *Hinkal) InitUserKeysFromSeedPhrases(seedPhrasesJSON string) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.InitUserKeysFromSeedPhrases(h.c.h, seedPhrasesJSON)
}

func (h *Hinkal) ResetMerkle(chainIDsJSON string) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.ResetMerkle(h.c.h, chainIDsJSON)
}

func (h *Hinkal) ResetMerkleTreesIfNecessary(chainIDsJSON string) error {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.ResetMerkleTreesIfNecessary(h.c.h, chainIDsJSON)
}

func (h *Hinkal) GetTotalBalance(chainID int64, userKeysSignature, ethAddress string, resetCache, useBlockedUtxos bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetTotalBalance(h.c.h, chainID, userKeysSignature, ethAddress, resetCache, useBlockedUtxos)
}

func (h *Hinkal) GetStuckShieldedBalances(chainID int64, userKeysSignature, ethAddress string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetStuckShieldedBalances(h.c.h, chainID, userKeysSignature, ethAddress)
}

func (h *Hinkal) GetSupportedChains() (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetSupportedChains(h.c.h)
}

func (h *Hinkal) GetShieldedPublicKey() (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetShieldedPublicKey(h.c.h)
}
