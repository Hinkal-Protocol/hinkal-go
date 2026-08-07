package solanautils

import (
	"context"
	"fmt"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	solanadata "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
)

const transactionFetchRetryCount = 30

func FetchTransactionWithRetry(ctx context.Context, chainID int, signature string) (*solanadata.Transaction, error) {
	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return nil, err
	}
	client := solanadata.NewClient(rpcURL)
	for attempt := 0; attempt < transactionFetchRetryCount; attempt++ {
		tx, err := client.GetTransaction(ctx, signature)
		if err == nil && tx != nil && tx.Meta != nil {
			if tx.Meta.Err != nil {
				return nil, fmt.Errorf("solanautils: Solana transaction failed: %v", tx.Meta.Err)
			}
			return tx, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("solanautils: Solana transaction %s missing", signature)
}

func TransactionEvents(meta *solanadata.TxMeta) []*solanadata.DecodedEvent {
	events := solanadata.ParseLogsForEvents(meta.LogMessages)
	var cpiData []string
	for _, inner := range meta.InnerInstructions {
		for _, ix := range inner.Instructions {
			if ix.Data != "" {
				cpiData = append(cpiData, ix.Data)
			}
		}
	}
	return append(events, solanadata.ParseCpiForEvents(cpiData)...)
}

func IntSliceArg(value any) ([]int, bool) {
	switch v := value.(type) {
	case []int:
		return v, true
	case []any:
		out := make([]int, len(v))
		for i, item := range v {
			n, ok := item.(float64)
			if !ok {
				return nil, false
			}
			out[i] = int(n)
		}
		return out, true
	default:
		return nil, false
	}
}

func IntMatrixArg(value any) ([][]int, bool) {
	switch v := value.(type) {
	case [][]int:
		return v, true
	case []any:
		out := make([][]int, len(v))
		for i, row := range v {
			converted, ok := IntSliceArg(row)
			if !ok {
				return nil, false
			}
			out[i] = converted
		}
		return out, true
	default:
		return nil, false
	}
}
