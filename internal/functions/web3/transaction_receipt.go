package web3

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
)

const receiptFetchRetryCount = 30

func TxHashToHash(txHash string) common.Hash {
	if strings.HasPrefix(txHash, "0x") || strings.HasPrefix(txHash, "0X") {
		return common.HexToHash(txHash)
	}
	return common.HexToHash("0x" + txHash)
}

func FetchTransactionReceiptWithRetry(ctx context.Context, client *ethclient.Client, txHash string) (*ethtypes.Receipt, error) {
	hash := TxHashToHash(txHash)
	for attempt := 0; attempt < receiptFetchRetryCount; attempt++ {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		if !errors.Is(err, ethereum.NotFound) && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("web3: receipt not found for %s", txHash)
}
