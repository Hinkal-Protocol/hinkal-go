package mobile

import (
	hinkal "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/hinkal"
)

func (h *Hinkal) FetchClaimableUtxos(chainID int64, ethAddress, signature string, isSolanaLedger bool, txMessageForSolanaLedger string) (string, error) {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.FetchClaimableUtxos(h.c.h, h.c.claimable, chainID, ethAddress, signature, isSolanaLedger, txMessageForSolanaLedger)
}

func (h *Hinkal) DecodeUtxosFromReceipt(chainID int64, receiptJSON, userKeysSignature, tokenAddr string) (string, error) {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.DecodeUtxosFromReceipt(h.c.claimable, chainID, receiptJSON, userKeysSignature, tokenAddr)
}

func (h *Hinkal) DecodeSolanaUtxosFromTransaction(transactionJSON, userKeysSignature, compressedMintAddress string) (string, error) {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.DecodeSolanaUtxosFromTransaction(h.c.claimable, transactionJSON, userKeysSignature, compressedMintAddress)
}

func (h *Hinkal) StoreUtxoInEnclave(chainID int64, senderAddress, recipientEthAddress, handle, claimableSignature string) error {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.StoreUtxoInEnclave(h.c.h, h.c.claimable, chainID, senderAddress, recipientEthAddress, handle, claimableSignature)
}
