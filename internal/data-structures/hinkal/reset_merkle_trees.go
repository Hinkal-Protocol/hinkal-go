package hinkal

import (
	"context"
	"errors"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/snapshot"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

var resetMerkleTreesMutexByChain sync.Map

func getResetMerkleTreesMutex(chainID int) *sync.Mutex {
	mutex, _ := resetMerkleTreesMutexByChain.LoadOrStore(chainID, &sync.Mutex{})
	m, ok := mutex.(*sync.Mutex)
	if !ok {
		panic("reset merkle trees mutex registry holds a non-*sync.Mutex value")
	}
	return m
}

func resetMerkleTrees(ctx context.Context, h *Hinkal, chainIDs ...int) error {
	chainsToReset := chainIDs
	if len(chainsToReset) == 0 {
		chainsToReset = h.GetSupportedChains()
	}

	var wg sync.WaitGroup
	errs := make([]error, len(chainsToReset))
	for i, chainID := range chainsToReset {
		wg.Add(1)
		go func(i int, chainID int) {
			defer wg.Done()
			errs[i] = resetMerkleTreeForChain(ctx, h, chainID)
		}(i, chainID)
	}
	wg.Wait()

	return errors.Join(errs...)
}

func resetMerkleTreeForChain(ctx context.Context, h *Hinkal, chainID int) error {
	mutex := getResetMerkleTreesMutex(chainID)
	mutex.Lock()
	defer mutex.Unlock()

	if h.generateProofRemotely && constants.IsEnclaveTxChain(chainID) {
		return nil
	}

	h.mu.RLock()
	oldCommitments := h.CommitmentsSnapshotServiceByChain[chainID]
	oldNullifiers := h.NullifierSnapshotServiceByChain[chainID]
	h.mu.RUnlock()
	if oldCommitments != nil {
		oldCommitments.IntervalClear()
	}
	if oldNullifiers != nil {
		oldNullifiers.IntervalClear()
	}

	hinkalAddress, err := constants.HinkalAddress(chainID)
	if err != nil {
		return err
	}
	fetcher := snapshot.NewSnapshotFetcherService(chainID, hinkalAddress)

	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return err
	}

	eventsFetchingMutex := utils.GetChainBalanceFetchingMutex(chainID)

	if constants.IsSolanaLike(chainID) {
		emitter := eventservice.NewSolanaBlockchainEventEmitter(chainID, rpcURL, hinkalAddress, 0, false, eventsFetchingMutex, nil)
		commitments := snapshot.NewClientSolanaCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
		nullifiers := snapshot.NewClientSolanaNullifierSnapshotService(emitter, fetcher)
		if err := commitments.Svc.Init(ctx); err != nil {
			return err
		}
		if err := nullifiers.Svc.Init(ctx); err != nil {
			return err
		}
		if err := emitter.Init(ctx); err != nil {
			return err
		}
		h.storeChainState(chainID, commitments, nullifiers)
		return nil
	}

	emitter, err := eventservice.NewEVMEmitter(chainID, rpcURL, hinkalAddress, 0, eventsFetchingMutex)
	if err != nil {
		return err
	}
	commitments := snapshot.NewClientCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
	nullifiers := snapshot.NewClientNullifierSnapshotService(emitter, fetcher)
	if err := commitments.Svc.Init(ctx); err != nil {
		return err
	}
	if err := nullifiers.Svc.Init(ctx); err != nil {
		return err
	}
	if err := emitter.Init(ctx); err != nil {
		return err
	}
	h.storeChainState(chainID, commitments, nullifiers)
	return nil
}
