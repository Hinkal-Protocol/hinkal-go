package enclave

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func StoreClaimableKeyInEnclave(
	ctx context.Context,
	senderAddress string,
	recipientEthAddress string,
	shieldedPrivateKey string,
	chainID int,
	claimableSignature string,
) error {
	payload, err := claimableKeyPayload(shieldedPrivateKey, senderAddress, claimableSignature)
	if err != nil {
		return err
	}

	handshake, err := MakeHandshakeAndEncrypt(ctx, payload)
	if err != nil {
		return err
	}

	_, err = api.StoreClaimableKeyEnclaveCall(ctx, recipientEthAddress, handshake.InputCiphertext, handshake.KeyCiphertext, chainID)
	return err
}

func GetUtxosFromEnclave(
	ctx context.Context,
	ethAddress string,
	signature string,
	chainID int,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	items, err := fetchUtxosFromEnclave(ctx, ethAddress, signature, chainID)
	if err != nil {
		log.Printf("failed to fetch utxos from gcp: %v", err)
		return []types.UtxoConstructorParamsWithSenderAddress{}, nil
	}
	return items, nil
}

func fetchUtxosFromEnclave(
	ctx context.Context,
	ethAddress string,
	signature string,
	chainID int,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	handshake, err := MakeHandshakeAndEncrypt(ctx, []byte(signature))
	if err != nil {
		return nil, err
	}

	raw, err := api.GetUtxosEnclaveCall(
		ctx,
		ethAddress,
		handshake.InputCiphertext,
		handshake.KeyCiphertext,
		chainID,
	)
	if err != nil {
		return nil, err
	}
	resp, err := OpenSealedResponse[api.GetUtxosResponse](raw, handshake.Key)
	if err != nil {
		return nil, err
	}

	items := make([]types.UtxoConstructorParamsWithSenderAddress, 0, len(resp.Utxos))
	for i, raw := range resp.Utxos {
		item, err := parseEnclaveUtxoResponse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse enclave UTXO %d: %w", i, err)
		}
		items = append(items, item)
	}

	return deduplicateUtxosByCommitment(items)
}

func claimableKeyPayload(shieldedPrivateKey, senderAddress, claimableSignature string) ([]byte, error) {
	if shieldedPrivateKey == "" {
		return nil, fmt.Errorf("claimable UTXO is missing its shielded private key")
	}
	return json.Marshal(struct {
		ShieldedPrivateKey string `json:"shieldedPrivateKey"`
		ClaimableSignature string `json:"claimableSignature,omitempty"`
		SenderAddress      string `json:"senderAddress"`
	}{
		ShieldedPrivateKey: shieldedPrivateKey,
		ClaimableSignature: claimableSignature,
		SenderAddress:      senderAddress,
	})
}

func parseEnclaveUtxoResponse(raw json.RawMessage) (types.UtxoConstructorParamsWithSenderAddress, error) {
	var params types.UtxoParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return types.UtxoConstructorParamsWithSenderAddress{}, err
	}

	var meta struct {
		SenderAddress      string `json:"senderAddress"`
		ClaimableSignature string `json:"claimableSignature"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return types.UtxoConstructorParamsWithSenderAddress{}, err
	}

	return types.UtxoConstructorParamsWithSenderAddress{
		UtxoParams:         params,
		SenderAddress:      meta.SenderAddress,
		ClaimableSignature: meta.ClaimableSignature,
	}, nil
}

func deduplicateUtxosByCommitment(items []types.UtxoConstructorParamsWithSenderAddress) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	seenCommitments := make(map[string]struct{}, len(items))
	unique := make([]types.UtxoConstructorParamsWithSenderAddress, 0, len(items))

	for _, item := range items {
		u, err := utxo.NewUtxo(item.UtxoParams)
		if err != nil {
			return nil, err
		}
		commitment, err := u.GetCommitment()
		if err != nil {
			return nil, err
		}
		if _, ok := seenCommitments[commitment]; ok {
			continue
		}
		seenCommitments[commitment] = struct{}{}
		unique = append(unique, item)
	}

	return unique, nil
}
