package api

import (
	"context"
	"encoding/json"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type TokenBalanceEntry struct {
	Erc20TokenAddress string `json:"erc20TokenAddress"`
	Amount            string `json:"amount"`
}

type ChainBalancesEntry struct {
	ChainID              int                 `json:"chainId"`
	ConfirmedBalances    []TokenBalanceEntry `json:"confirmedBalances"`
	PreConfirmedBalances []TokenBalanceEntry `json:"preConfirmedBalances"`
	Error                string              `json:"error,omitempty"`
}

type GetBalancesEnclaveResponse struct {
	Results []ChainBalancesEntry `json:"results"`
}

func GetBalancesEnclaveCall(
	ctx context.Context,
	chainIDs []int,
	keyCiphertext, inputCiphertext string,
	useBlockedUtxos bool,
	hashedEthereumAddress string,
) (json.RawMessage, error) {
	body := map[string]any{
		"chainIds":        chainIDs,
		"input":           inputCiphertext,
		"key":             keyCiphertext,
		"useBlockedUtxos": useBlockedUtxos,
	}
	if hashedEthereumAddress != "" {
		body["hashedEthereumAddress"] = hashedEthereumAddress
	}
	var raw json.RawMessage
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.GetBalances, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type DecryptUtxoEnclaveResponse struct {
	Utxos            []types.UtxoParams               `json:"utxos"`
	EncryptedOutputs []*types.EncryptedOutputWithSign `json:"encryptedOutputs"`
	LastOutput       string                           `json:"lastOutput"`
}

func DecryptUtxoEnclaveCall(ctx context.Context, chainID int, keyCiphertext, inputCiphertext string) (*DecryptUtxoEnclaveResponse, error) {
	body := map[string]any{
		"chainId": chainID,
		"input":   inputCiphertext,
		"key":     keyCiphertext,
	}
	var resp DecryptUtxoEnclaveResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.DecryptUtxos, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type StoreClaimableKeyResponse struct {
	Status string `json:"status"`
}

func StoreClaimableKeyEnclaveCall(ctx context.Context, encryptedMaterial, key string) (*StoreClaimableKeyResponse, error) {
	body := map[string]any{
		"encryptedMaterial": encryptedMaterial,
		"key":               key,
	}
	var resp StoreClaimableKeyResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.StoreClaimableKey, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type GetUtxosResponse struct {
	Utxos []json.RawMessage `json:"utxos"`
}

func GetUtxosEnclaveCall(
	ctx context.Context,
	ethAddress, encryptedSignature, key string,
	chainID int,
) (json.RawMessage, error) {
	body := map[string]any{
		"ethAddress":         ethAddress,
		"encryptedSignature": encryptedSignature,
		"key":                key,
		"chainId":            chainID,
	}
	var raw json.RawMessage
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.GetUtxos, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type StoreAndGetSignatureResponse struct {
	Signature string `json:"signature"`
}

func StoreAndGetSignatureEnclaveCall(
	ctx context.Context,
	ethAddress, encryptedSignature, key string,
) (json.RawMessage, error) {
	body := map[string]any{
		"ethAddress":         ethAddress,
		"encryptedSignature": encryptedSignature,
		"key":                key,
	}
	var raw json.RawMessage
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.StoreAndGetSignature, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type SignProofRequest struct {
	ChainID          int        `json:"chainId"`
	A                []string   `json:"a"`
	B                [][]string `json:"b"`
	C                []string   `json:"c"`
	Inputs           []string   `json:"inputs"`
	ExternalActionID string     `json:"externalActionId"`
	TokenNumber      int        `json:"tokenNumber"`
	NullifierAmount  int        `json:"nullifierAmount"`
	OutputAmount     int        `json:"outputAmount"`
	SkipLock         bool       `json:"skipLock"`
}

type signProofResponse struct {
	Signature string `json:"signature"`
}

func SignProofEnclaveCall(ctx context.Context, req SignProofRequest) (string, error) {
	var resp signProofResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.SignProof, req, &resp); err != nil {
		return "", err
	}
	return resp.Signature, nil
}

func GenerateProofsEnclaveCall(
	ctx context.Context,
	wasmFilenames, zkeyFilenames []string,
	inputCiphertext, keyCiphertext string,
) ([]types.GenerateProofResponseType, error) {
	body := map[string]any{
		"circuit_wasms": wasmFilenames,
		"circuit_zkeys": zkeyFilenames,
		"inputs":        inputCiphertext,
		"key":           keyCiphertext,
	}
	var resp []types.GenerateProofResponseType
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.GenerateProofs, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
