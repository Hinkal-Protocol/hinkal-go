package api

import (
	"context"
	"fmt"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SnapshotServerEventsResponse struct {
	Events            []string `json:"events"`
	LatestBlockNumber uint64   `json:"latestBlockNumber"`
}

func GetSnapshotServerEvents(ctx context.Context, chainID int, eventCategory types.EventCategory, fromBlockNumber uint64) (*SnapshotServerEventsResponse, error) {
	url := fmt.Sprintf("%s%s?chainId=%d&eventCategory=%s&fromBlockNumber=%d",
		constants.GetSnapshotServerURL(), constants.SnapshotServerConfig.Events, chainID, eventCategory, fromBlockNumber)
	var resp SnapshotServerEventsResponse
	if err := Get(ctx, url, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func FetchSnapshots(ctx context.Context, chainID int) (*types.SnapshotsResponse, error) {
	var urlsResp struct {
		Commitments struct {
			URL           string `json:"url"`
			HinkalAddress string `json:"hinkalAddress"`
			ChainID       int    `json:"chainId"`
		} `json:"commitments"`
		Nullifiers struct {
			URL string `json:"url"`
		} `json:"nullifiers"`
	}
	url := constants.GetSnapshotServerURL() + constants.SnapshotServerConfig.GetSnapshots(chainID)
	if err := Get(ctx, url, &urlsResp); err != nil {
		return nil, fmt.Errorf("get snapshot URLs: %w", err)
	}

	var (
		commitments types.CommitmentsTreeSnapshot
		nullifiers  types.NullifiersSnapshot
		wg          sync.WaitGroup
		errs        = make([]error, 2)
	)
	wg.Add(2)
	go func() { defer wg.Done(); errs[0] = Get(ctx, urlsResp.Commitments.URL, &commitments) }()
	go func() { defer wg.Done(); errs[1] = Get(ctx, urlsResp.Nullifiers.URL, &nullifiers) }()
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	commitments.HinkalAddress = urlsResp.Commitments.HinkalAddress
	commitments.ChainID = urlsResp.Commitments.ChainID

	return &types.SnapshotsResponse{
		Commitments: commitments,
		Nullifiers:  nullifiers,
	}, nil
}
