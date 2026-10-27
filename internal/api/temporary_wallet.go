package api

import (
	"context"
	"encoding/json"
	"log"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type TemporaryWalletNonceEntry struct {
	Nonce               int                                      `json:"nonce"`
	RecoveryDestination types.TemporaryWalletRecoveryDestination `json:"recoveryDestination"`
	IsLegacy            bool                                     `json:"isLegacy"`
	CreatedAt           string                                   `json:"createdAt,omitempty"`
}

type TemporaryWalletNoncesResponse struct {
	NonceEntries []TemporaryWalletNonceEntry
}

type rawTemporaryWalletNonceEntry struct {
	TemporaryWalletNonceEntry
	Nonce json.Number `json:"nonce"`
}

type rawTemporaryWalletNoncesResponse struct {
	NonceEntries []rawTemporaryWalletNonceEntry `json:"nonceEntries"`
}

type TemporaryWalletNonceRequest struct {
	HashedEthereumAddress string                                   `json:"hashedEthereumAddress"`
	ChainID               int                                      `json:"chainId"`
	Nonce                 int                                      `json:"nonce"`
	RecoveryDestination   types.TemporaryWalletRecoveryDestination `json:"recoveryDestination,omitempty"`
	UsesAuxKey            bool                                     `json:"usesAuxKey,omitempty"`
}

type TemporaryWalletNonceResponse struct {
	Success bool `json:"success"`
}

func GetTemporaryWalletNonces(ctx context.Context, chainID int, hashedEthereumAddress string) (TemporaryWalletNoncesResponse, error) {
	var raw rawTemporaryWalletNoncesResponse
	url := constants.GetServerURL() + constants.ServerConfig.GetTemporaryWalletNonces(hashedEthereumAddress, chainID)
	if err := Get(ctx, url, &raw); err != nil {
		return TemporaryWalletNoncesResponse{}, err
	}

	entries := make([]TemporaryWalletNonceEntry, 0, len(raw.NonceEntries))
	for _, entry := range raw.NonceEntries {
		nonce, err := entry.Nonce.Int64()
		if err != nil || nonce < 0 {
			log.Printf("Skipping invalid temporary wallet nonce %s on chain %d", entry.Nonce, chainID)
			continue
		}
		entry.TemporaryWalletNonceEntry.Nonce = int(nonce)
		entries = append(entries, entry.TemporaryWalletNonceEntry)
	}
	return TemporaryWalletNoncesResponse{NonceEntries: entries}, nil
}

func AddTemporaryWalletNonce(ctx context.Context, chainID int, hashedEthereumAddress string, nonce int, recoveryDestination types.TemporaryWalletRecoveryDestination) (TemporaryWalletNonceResponse, error) {
	var resp TemporaryWalletNonceResponse
	url := constants.GetServerURL() + constants.ServerConfig.AddTemporaryWalletNonce
	if err := Post(ctx, url, TemporaryWalletNonceRequest{
		HashedEthereumAddress: hashedEthereumAddress,
		ChainID:               chainID,
		Nonce:                 nonce,
		RecoveryDestination:   recoveryDestination,
		UsesAuxKey:            true,
	}, &resp); err != nil {
		return TemporaryWalletNonceResponse{}, err
	}
	return resp, nil
}

func RemoveTemporaryWalletNonce(ctx context.Context, chainID int, hashedEthereumAddress string, nonce int) (TemporaryWalletNonceResponse, error) {
	var resp TemporaryWalletNonceResponse
	url := constants.GetServerURL() + constants.ServerConfig.RemoveTemporaryWalletNonce
	if err := Post(ctx, url, TemporaryWalletNonceRequest{
		HashedEthereumAddress: hashedEthereumAddress,
		ChainID:               chainID,
		Nonce:                 nonce,
	}, &resp); err != nil {
		return TemporaryWalletNonceResponse{}, err
	}
	return resp, nil
}
