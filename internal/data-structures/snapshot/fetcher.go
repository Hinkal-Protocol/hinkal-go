package snapshot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const defaultCacheDuration = 5 * time.Minute

type SnapshotFetcherService struct {
	chainID         int
	contractAddress string
	cacheDuration   time.Duration
	mu              sync.Mutex
	cached          *types.SnapshotsResponse
	lastFetchedAt   time.Time
}

func NewSnapshotFetcherService(chainID int, contractAddress string, cacheDuration ...time.Duration) *SnapshotFetcherService {
	dur := defaultCacheDuration
	if len(cacheDuration) > 0 {
		dur = cacheDuration[0]
	}
	return &SnapshotFetcherService{chainID: chainID, contractAddress: contractAddress, cacheDuration: dur}
}

func (s *SnapshotFetcherService) GetAll(ctx context.Context, forceRefresh ...bool) (*types.SnapshotsResponse, error) {
	return s.ensureSnapshots(ctx, len(forceRefresh) > 0 && forceRefresh[0])
}

func (s *SnapshotFetcherService) GetCommitments(ctx context.Context, forceRefresh ...bool) (*types.CommitmentsSerializedSnapshot, error) {
	all, err := s.ensureSnapshots(ctx, len(forceRefresh) > 0 && forceRefresh[0])
	if err != nil {
		return nil, err
	}
	block := all.Commitments.LatestBlockNumber
	return &types.CommitmentsSerializedSnapshot{
		LatestBlockNumber:            &block,
		MerkleTree:                   &all.Commitments.MerkleTree,
		EncryptedOutputs:             all.Commitments.EncryptedOutputs,
		EncryptedOutputsByCommitment: all.Commitments.EncryptedOutputsByCommitment,
	}, nil
}

func (s *SnapshotFetcherService) GetNullifiers(ctx context.Context, forceRefresh ...bool) (*types.NullifierSerializedSnapshot, error) {
	all, err := s.ensureSnapshots(ctx, len(forceRefresh) > 0 && forceRefresh[0])
	if err != nil {
		return nil, err
	}
	block := all.Nullifiers.LatestBlockNumber
	return &types.NullifierSerializedSnapshot{LatestBlockNumber: &block, Nullifiers: all.Nullifiers.Nullifiers}, nil
}

func (s *SnapshotFetcherService) ensureSnapshots(ctx context.Context, force bool) (*types.SnapshotsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if force || s.cached == nil || time.Since(s.lastFetchedAt) > s.cacheDuration {
		snapshots, err := api.FetchSnapshots(ctx, s.chainID)
		if err != nil {
			return nil, fmt.Errorf("fetch snapshots: %w", err)
		}
		if !strings.EqualFold(snapshots.Commitments.HinkalAddress, s.contractAddress) ||
			snapshots.Commitments.ChainID != s.chainID {
			return nil, fmt.Errorf("snapshots: incorrect contract address or chain ID")
		}
		s.cached = snapshots
		s.lastFetchedAt = time.Now()
	}
	return s.cached, nil
}
