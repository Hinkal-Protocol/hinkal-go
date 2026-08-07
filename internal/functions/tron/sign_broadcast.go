package tron

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type SignTxHashFunc func(ctx context.Context, txHash []byte) ([]byte, error)

func SignAndBroadcast(ctx context.Context, wc *WalletClient, sign SignTxHashFunc, tx *Transaction) (string, error) {
	raw, err := hex.DecodeString(tx.RawDataHex)
	if err != nil {
		return "", fmt.Errorf("decode tron raw_data_hex: %w", err)
	}
	h := sha256.Sum256(raw)
	sig, err := sign(ctx, h[:])
	if err != nil {
		return "", fmt.Errorf("sign tron transaction: %w", err)
	}
	tx.Signature = []string{hex.EncodeToString(sig)}

	res, err := wc.BroadcastTransaction(ctx, tx)
	if err != nil {
		return "", err
	}
	if !res.Result {
		return "", fmt.Errorf("tron broadcast rejected: %s", res.message())
	}
	return hex.EncodeToString(h[:]), nil
}

func WaitForTransaction(ctx context.Context, wc *WalletClient, txHash string, confirmations uint64) (bool, error) {
	for {
		info, err := wc.GetTransactionInfoByID(ctx, txHash)
		if err == nil && info.ID != "" {
			if info.Result == "FAILED" {
				message := info.ResMessage
				if decoded, derr := hex.DecodeString(message); derr == nil && len(decoded) > 0 {
					message = string(decoded)
				}
				if message == "" {
					message = "FAILED"
				}
				return false, fmt.Errorf("tron transaction failed: %s", message)
			}
			if confirmations <= 1 {
				return true, nil
			}
			targetBlock := info.BlockNumber + int64(confirmations) - 1
			for {
				number, berr := wc.GetNowBlockNumber(ctx)
				if berr == nil && number >= targetBlock {
					return true, nil
				}
				select {
				case <-ctx.Done():
					return false, fmt.Errorf("timeout waiting for Tron confirmations on %s: %w", txHash, ctx.Err())
				case <-time.After(2 * time.Second):
				}
			}
		}

		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for Tron transaction %s: %w", txHash, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}
