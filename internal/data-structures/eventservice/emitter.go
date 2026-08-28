package eventservice

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
)

const (
	retryMinPageSize = 100
	maxRetries       = 20
)

type EmitterDelegate interface {
	Clear()
	StartUpdateListener(ctx context.Context, emitter *BlockchainEventEmitter)
}

type BlockchainEventEmitter struct {
	chainID           int
	client            *ethclient.Client
	contractAddr      common.Address
	contractABI       abi.ABI
	latestBlockNumber watermark
	maxPageSize       *uint64
	isServer          bool
	isReady           bool
	inProgress        bool

	eventsFetchingMutex *sync.RWMutex
	eventProcessors     []blockchainevent.EventProcessorFunc
	OnEventsProcessed   func(count int)
	delegate            EmitterDelegate
}

func New(
	chainID int,
	client *ethclient.Client,
	contractAddr common.Address,
	contractABI abi.ABI,
	initialBlockNumber uint64,
	isServer bool,
	delegate EmitterDelegate,
	eventsFetchingMutex *sync.RWMutex,
	maxPageSize *uint64,
) *BlockchainEventEmitter {
	if eventsFetchingMutex == nil {
		eventsFetchingMutex = &sync.RWMutex{}
	}
	e := &BlockchainEventEmitter{
		chainID:             chainID,
		client:              client,
		contractAddr:        contractAddr,
		contractABI:         contractABI,
		isServer:            isServer,
		delegate:            delegate,
		eventsFetchingMutex: eventsFetchingMutex,
		maxPageSize:         maxPageSize,
	}
	e.latestBlockNumber.store(initialBlockNumber)
	return e
}

func (e *BlockchainEventEmitter) ChainID() int { return e.chainID }

func (e *BlockchainEventEmitter) LatestBlockNumber() uint64 {
	return e.latestBlockNumber.load()
}

func (e *BlockchainEventEmitter) IsReady() bool { return e.isReady }

func (e *BlockchainEventEmitter) AdvanceLatestBlockNumber(blockNumber uint64) {
	e.latestBlockNumber.advance(blockNumber)
}

func (e *BlockchainEventEmitter) ProcessExternalEvents(events []*blockchainevent.BlockchainEvent, latestBlock uint64) error {
	e.eventsFetchingMutex.Lock()
	defer e.eventsFetchingMutex.Unlock()
	if err := e.processEvents(events, latestBlock); err != nil {
		return err
	}
	e.AdvanceLatestBlockNumber(latestBlock)
	return nil
}

func (e *BlockchainEventEmitter) SyncFromAtMost(blockNumber uint64) {
	e.latestBlockNumber.syncFromAtMost(blockNumber)
}

func (e *BlockchainEventEmitter) AddEventProcessorFunction(fn blockchainevent.EventProcessorFunc) {
	if e.isReady {
		panic("cannot add processor after Init")
	}
	e.eventProcessors = append(e.eventProcessors, fn)
}

func (e *BlockchainEventEmitter) IntervalClear() {
	e.isReady = false
	e.eventProcessors = nil
	e.delegate.Clear()
}

func (e *BlockchainEventEmitter) Init(ctx context.Context) error {
	if e.isReady {
		return fmt.Errorf("already initialized")
	}
	e.isReady = true
	if err := e.RetrieveEvents(ctx, e.LatestBlockNumber()+1, false); err != nil {
		return err
	}
	// The update listener outlives this call (stopped only by IntervalClear), so detach it
	// from the caller's cancellation/deadline while preserving context values.
	go e.delegate.StartUpdateListener(context.WithoutCancel(ctx), e)
	return nil
}

func (e *BlockchainEventEmitter) GetEventsInRange(ctx context.Context, from, to uint64) ([]*blockchainevent.BlockchainEvent, error) {
	pages := buildPages(from, to, e.maxPageSize)
	addrs := []common.Address{e.contractAddr}
	if hook, ok := blockedUtxosHookAddress(e.chainID); ok {
		addrs = append(addrs, hook)
	}
	var all []*blockchainevent.BlockchainEvent
	for _, p := range pages {
		for _, addr := range addrs {
			evs, err := e.getEventsForSingleContract(ctx, addr, p[0], p[1], 0)
			if err != nil {
				return nil, err
			}
			all = append(all, evs...)
		}
	}
	return all, nil
}

func blockedUtxosHookAddress(chainID int) (common.Address, bool) {
	if constants.IsTronLike(chainID) {
		return common.Address{}, false
	}
	addr, err := constants.DepositOnChainUtxosAddress(chainID)
	if err != nil || addr == "" {
		return common.Address{}, false
	}
	return common.HexToAddress(addr), true
}

func (e *BlockchainEventEmitter) GetLastBlockNumberForEventRequest(ctx context.Context) (uint64, error) {
	latest, err := e.client.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	if !e.isServer {
		return latest, nil
	}
	reorgDepth, err := constants.GetReorgDepth(e.chainID)
	if err != nil {
		return 0, err
	}
	safe := safeReorgBlock(latest, reorgDepth)
	if cur := e.LatestBlockNumber(); cur > safe {
		return cur, nil
	}
	return safe, nil
}

func (e *BlockchainEventEmitter) RetrieveEvents(ctx context.Context, fromBlock uint64, force bool) error {
	e.eventsFetchingMutex.Lock()
	defer e.eventsFetchingMutex.Unlock()
	if e.inProgress && !force {
		return nil
	}
	e.inProgress = true
	defer func() { e.inProgress = false }()

	lastBlock, err := e.GetLastBlockNumberForEventRequest(ctx)
	if err != nil {
		return fmt.Errorf("get last block: %w", err)
	}
	if lastBlock < fromBlock {
		return nil
	}
	events, err := e.GetEventsInRange(ctx, fromBlock, lastBlock)
	if err != nil {
		return fmt.Errorf("get events in range: %w", err)
	}
	if err := e.processEvents(events, lastBlock); err != nil {
		return err
	}
	e.latestBlockNumber.store(lastBlock)
	return nil
}

func (e *BlockchainEventEmitter) getEventsForSingleContract(
	ctx context.Context, addr common.Address, from, to uint64, retry int,
) ([]*blockchainevent.BlockchainEvent, error) {
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{addr},
	}
	logs, err := e.client.FilterLogs(ctx, query)
	if err != nil {
		if retry < maxRetries && to-from > retryMinPageSize {
			mid := (from + to) / 2
			a, err := e.getEventsForSingleContract(ctx, addr, from, mid, retry+1)
			if err != nil {
				return nil, err
			}
			b, err := e.getEventsForSingleContract(ctx, addr, mid+1, to, retry+1)
			if err != nil {
				return nil, err
			}
			return append(a, b...), nil
		}
		return nil, err
	}
	events := make([]*blockchainevent.BlockchainEvent, 0, len(logs))
	for _, log := range logs {
		ev, err := blockchainevent.NewFromLog(log, e.contractABI)
		if err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func (e *BlockchainEventEmitter) processEvents(events []*blockchainevent.BlockchainEvent, scannedToBlock uint64) error {
	counts := make([]int, len(e.eventProcessors))
	var g errgroup.Group
	for i, proc := range e.eventProcessors {
		g.Go(func() error {
			n, err := proc(events, &scannedToBlock)
			if err != nil {
				return err
			}
			counts[i] = n
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if e.OnEventsProcessed != nil {
		e.OnEventsProcessed(total)
	}
	return nil
}

func buildPages(from, to uint64, maxPage *uint64) [][2]uint64 {
	if maxPage == nil || *maxPage == 0 {
		return [][2]uint64{{from, to}}
	}
	var pages [][2]uint64
	for cur := from; cur <= to; cur += *maxPage {
		end := min(cur+*maxPage-1, to)
		pages = append(pages, [2]uint64{cur, end})
	}
	return pages
}
