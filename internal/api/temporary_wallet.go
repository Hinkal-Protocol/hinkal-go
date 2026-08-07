package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type TemporaryWalletNoncesResponse struct {
	Nonces []int `json:"nonces"`
}

type TemporaryWalletNonceRequest struct {
	HashedEthereumAddress string                                   `json:"hashedEthereumAddress"`
	ChainID               int                                      `json:"chainId"`
	Nonce                 int                                      `json:"nonce"`
	RecoveryDestination   types.TemporaryWalletRecoveryDestination `json:"recoveryDestination,omitempty"`
}

type TemporaryWalletNonceResponse struct {
	Success bool `json:"success"`
}

func GetTemporaryWalletNonces(ctx context.Context, chainID int, hashedEthereumAddress string) (TemporaryWalletNoncesResponse, error) {
	var resp TemporaryWalletNoncesResponse
	url := constants.GetServerURL() + constants.ServerConfig.GetTemporaryWalletNonces(hashedEthereumAddress, chainID)
	if err := Get(ctx, url, &resp); err != nil {
		return TemporaryWalletNoncesResponse{}, err
	}
	return resp, nil
}

func AddTemporaryWalletNonce(ctx context.Context, chainID int, hashedEthereumAddress string, nonce int, recoveryDestination types.TemporaryWalletRecoveryDestination) (TemporaryWalletNonceResponse, error) {
	var resp TemporaryWalletNonceResponse
	url := constants.GetServerURL() + constants.ServerConfig.AddTemporaryWalletNonce
	if err := Post(ctx, url, TemporaryWalletNonceRequest{
		HashedEthereumAddress: hashedEthereumAddress,
		ChainID:               chainID,
		Nonce:                 nonce,
		RecoveryDestination:   recoveryDestination,
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
