package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
)

const evmReceiptPollInterval = 2 * time.Second

func waitForTransaction(ctx context.Context, client *ethclient.Client, txHash string, confirmations uint64) (bool, error) {
	if !strings.HasPrefix(txHash, "0x") {
		txHash = "0x" + txHash
	}
	hash := common.HexToHash(txHash)

	receipt, err := waitForTransactionReceipt(ctx, client, hash, txHash)
	if err != nil {
		return false, err
	}
	if receipt.Status != 1 {
		return false, errorhandling.ErrTransactionNotConfirmed
	}
	if confirmations <= 1 {
		return true, nil
	}

	for {
		current, err := client.TransactionReceipt(ctx, hash)
		if err != nil {
			return false, fmt.Errorf("transaction %s evicted from canonical chain: %w", txHash, err)
		}
		head, err := client.BlockNumber(ctx)
		if err == nil && head >= current.BlockNumber.Uint64()+confirmations-1 {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for confirmations on %s: %w", txHash, ctx.Err())
		case <-time.After(evmReceiptPollInterval):
		}
	}
}

func WaitForEVMTransaction(ctx context.Context, client *ethclient.Client, txHash string, confirmations uint64) (bool, error) {
	return waitForTransaction(ctx, client, txHash, confirmations)
}

func waitForTransactionReceipt(ctx context.Context, client *ethclient.Client, hash common.Hash, txHash string) (*ethtypes.Receipt, error) {
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		if !errors.Is(err, ethereum.NotFound) {
			return nil, fmt.Errorf("fetch transaction receipt %s: %w", txHash, err)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction receipt %s: %w", txHash, ctx.Err())
		case <-time.After(evmReceiptPollInterval):
		}
	}
}

func waitForSolanaTransaction(ctx context.Context, client *rpc.Client, txHash string) (bool, error) {
	sig, err := solana.SignatureFromBase58(txHash)
	if err != nil {
		return false, fmt.Errorf("parse solana signature %s: %w", txHash, err)
	}

	// Solana transactions expire after ~150 slots (~60s). Apply a hard deadline
	// if the caller did not already set one so we never loop forever.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}

	for {
		statuses, err := client.GetSignatureStatuses(ctx, false, sig)
		if err != nil {
			return false, fmt.Errorf("get signature status %s: %w", txHash, err)
		}
		if len(statuses.Value) > 0 && statuses.Value[0] != nil {
			status := statuses.Value[0]
			if status.Err != nil {
				return false, errorhandling.ErrTransactionNotConfirmed
			}
			if status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
				status.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return true, nil
			}
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for confirmation of %s: %w", txHash, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func hexSignature(sig []byte) (string, error) {
	if len(sig) == 0 {
		return "", errorhandling.ErrSigningFailed
	}
	return "0x" + common.Bytes2Hex(sig), nil
}
