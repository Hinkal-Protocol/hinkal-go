package solanautils

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const sendRetryCount = 10

var errConfirmationTimeout = errors.New("Solana transaction confirmation timed out")

type TransactionSender interface {
	SignAndSend(context.Context, *solana.Transaction) (solana.Signature, error)
}

func retryableSendError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errConfirmationTimeout) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "blockhash not found") ||
		strings.Contains(message, "blockhashnotfound") ||
		strings.Contains(message, "transactionexpiredblockheightexceedederror") ||
		strings.Contains(message, "block height exceeded")
}

func waitForConfirmation(
	ctx context.Context,
	connection *rpc.Client,
	signature solana.Signature,
	lastValidBlockHeight uint64,
) error {
	for {
		height, err := connection.GetBlockHeight(ctx, rpc.CommitmentConfirmed)
		if err == nil && height > lastValidBlockHeight {
			return errConfirmationTimeout
		}
		statuses, err := connection.GetSignatureStatuses(ctx, false, signature)
		if err == nil && len(statuses.Value) > 0 && statuses.Value[0] != nil {
			status := statuses.Value[0]
			if status.Err != nil {
				return fmt.Errorf("Solana transaction failed: %v", status.Err)
			}
			if status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
				status.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func SignAndConfirm(
	ctx context.Context,
	connection *rpc.Client,
	sender TransactionSender,
	build func(solana.Hash) (*solana.Transaction, error),
) (string, error) {
	for attempt := 0; attempt < sendRetryCount; attempt++ {
		latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
		if err != nil {
			return "", err
		}
		tx, err := build(latest.Value.Blockhash)
		if err != nil {
			return "", err
		}
		signature, err := sender.SignAndSend(ctx, tx)
		if err == nil {
			err = waitForConfirmation(ctx, connection, signature, latest.Value.LastValidBlockHeight)
			if err == nil {
				return signature.String(), nil
			}
		}
		if attempt == sendRetryCount-1 || !retryableSendError(err) {
			return "", err
		}
	}
	return "", errors.New("Solana transaction failed")
}
