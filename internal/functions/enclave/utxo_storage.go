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

func StoreUtxoInEnclave(
	ctx context.Context,
	senderAddress string,
	recipientEthAddress string,
	u *utxo.Utxo,
	chainID int,
	claimableSignature string,
) error {
	payload, err := enclaveUtxoPayload(u.GetConstructableParams(), senderAddress, claimableSignature)
	if err != nil {
		return err
	}

	keyCiphertext, inputCiphertext, err := MakeHandshakeAndEncrypt(ctx, payload)
	if err != nil {
		return err
	}

	_, err = api.StoreUtxoEnclaveCall(ctx, recipientEthAddress, inputCiphertext, keyCiphertext, chainID)
	return err
}

func GetUtxosFromEnclave(
	ctx context.Context,
	ethAddress string,
	signature string,
	chainID int,
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	items, err := fetchUtxosFromEnclave(ctx, ethAddress, signature, chainID, isSolanaLedger, txMessageForSolanaLedger)
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
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	keyCiphertext, inputCiphertext, err := MakeHandshakeAndEncrypt(ctx, []byte(signature))
	if err != nil {
		return nil, err
	}

	resp, err := api.GetUtxosEnclaveCall(
		ctx,
		ethAddress,
		inputCiphertext,
		keyCiphertext,
		chainID,
		isSolanaLedger,
		txMessageForSolanaLedger,
	)
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

func enclaveUtxoPayload(params types.UtxoParams, senderAddress, claimableSignature string) ([]byte, error) {
	base, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(base, &payload); err != nil {
		return nil, err
	}
	payload["senderAddress"] = senderAddress
	if claimableSignature != "" {
		payload["claimableSignature"] = claimableSignature
	}
	return json.Marshal(payload)
}

func parseEnclaveUtxoResponse(raw json.RawMessage) (types.UtxoConstructorParamsWithSenderAddress, error) {
	var params types.UtxoParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return types.UtxoConstructorParamsWithSenderAddress{}, err
	}

	var meta struct {
		SenderAddress      string `json:"senderAddress"`
		ClaimableSignature string `json:"claimableSignature"`
		ShieldedPrivateKey string `json:"shieldedPrivateKey"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return types.UtxoConstructorParamsWithSenderAddress{}, err
	}

	return types.UtxoConstructorParamsWithSenderAddress{
		UtxoParams:         params,
		SenderAddress:      meta.SenderAddress,
		ClaimableSignature: meta.ClaimableSignature,
		ShieldedPrivateKey: meta.ShieldedPrivateKey,
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
