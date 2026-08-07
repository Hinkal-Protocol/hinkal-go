package hinkal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/cache"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	_ ihinkal.IHinkal        = (*Hinkal)(nil)
	_ ihinkal.HinkalInternal = (*Hinkal)(nil)
)

type Hinkal struct {
	UserKeys *cryptokeys.UserKeys

	MerkleTreeHinkalByChain           map[int]merkletree.MerkleTree
	NullifiersByChain                 map[int]map[string]struct{}
	EncryptedOutputsByChain           map[int][]*types.EncryptedOutputWithSign
	CommitmentsSnapshotServiceByChain map[int]commitmentsSnapshot
	NullifierSnapshotServiceByChain   map[int]nullifierSnapshot

	ethereumProviderAdapter types.IProviderAdapter
	solanaProviderAdapter   types.IProviderAdapter
	tronProviderAdapter     types.IProviderAdapter

	generateProofRemotely               bool
	disableMerkleTreeUpdates            bool
	allowParallelBalanceLocalDecryption bool
	cacheDevice                         types.ICacheDevice
	tronChainID                         int

	mu                          sync.RWMutex
	balanceFetchingMutexByChain sync.Map // map[int]*sync.Mutex

	lastCallMu    sync.Mutex
	lastCallState map[string]rateLimitCall
}

type rateLimitCall struct {
	timestamp time.Time
	argsKey   string
}

const rateLimitInterval = time.Second

func NewHinkal(config *types.HinkalConfig) *Hinkal {
	cacheDevice := types.ICacheDevice(cache.NewMemoryCacheDevice())
	if config != nil {
		switch {
		case config.CacheDevice != nil:
			cacheDevice = config.CacheDevice
		case config.DisableCaching:
			cacheDevice = cache.NewNoopCacheDevice()
		case config.UseFileCache:
			fileCache := cache.NewFileCacheDevice(config.CacheFilePath)
			for key, value := range config.SerializedCache {
				fileCache.Set(key, value)
			}
			cacheDevice = fileCache
		case len(config.SerializedCache) > 0:
			cacheDevice = cache.NewMemoryCacheDeviceWithSerialized(config.SerializedCache)
		}
	}

	h := &Hinkal{
		UserKeys:                          cryptokeys.NewUserKeys(""),
		MerkleTreeHinkalByChain:           map[int]merkletree.MerkleTree{},
		NullifiersByChain:                 map[int]map[string]struct{}{},
		EncryptedOutputsByChain:           map[int][]*types.EncryptedOutputWithSign{},
		CommitmentsSnapshotServiceByChain: map[int]commitmentsSnapshot{},
		NullifierSnapshotServiceByChain:   map[int]nullifierSnapshot{},
		generateProofRemotely:             true,
		cacheDevice:                       cacheDevice,
		tronChainID:                       constants.CurrentTronChainID(),
		lastCallState:                     map[string]rateLimitCall{},
	}
	if config != nil {
		if config.GenerateProofRemotely != nil {
			h.generateProofRemotely = *config.GenerateProofRemotely
		}
		h.disableMerkleTreeUpdates = config.DisableMerkleTreeUpdates
		h.allowParallelBalanceLocalDecryption = config.AllowParallelBalanceLocalDecryption
		if config.TronChainOverride != 0 {
			h.tronChainID = config.TronChainOverride
		}
	}
	return h
}

func (h *Hinkal) InitUserKeysWithSignature(signature string) {
	h.UserKeys = cryptokeys.NewUserKeys(signature)
}

func (h *Hinkal) InitUserKeysFromSeedPhrases(seedPhrases []string) error {
	seedHash, err := utils.GenerateHashFromSeedPhrases(seedPhrases)
	if err != nil {
		return err
	}
	h.UserKeys = cryptokeys.NewUserKeys(seedHash)
	return nil
}

func (h *Hinkal) ResetMerkle(ctx context.Context, chainIDs ...int) error {
	if h.disableMerkleTreeUpdates {
		return nil
	}
	for _, chainID := range chainIDs {
		if !constants.IsHinkalSupportedChain(chainID) {
			return nil
		}
	}
	return resetMerkleTrees(ctx, h, chainIDs...)
}

func (h *Hinkal) ResetMerkleTreesIfNecessary(ctx context.Context, chainIDsToCheck ...int) error {
	if h.disableMerkleTreeUpdates {
		return nil
	}
	chains := chainIDsToCheck
	if len(chains) == 0 {
		chains = h.GetSupportedChains()
	}
	for _, chainID := range chains {
		if !constants.IsHinkalSupportedChain(chainID) {
			return nil
		}
	}

	needsReset := make([]bool, len(chains))
	var g errgroup.Group
	for i, chainID := range chains {
		g.Go(func() error {
			hinkalRootHash, err := h.GetHinkalTreeRootHash(ctx, chainID)
			if err != nil {
				return err
			}
			tree := h.merkleTree(chainID)
			if tree == nil {
				needsReset[i] = true
				return nil
			}
			localRootHash, err := tree.GetRootHash()
			if err != nil {
				return err
			}
			needsReset[i] = hinkalRootHash.Cmp(localRootHash) != 0
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	var chainsToReset []int
	for i, chainID := range chains {
		if needsReset[i] {
			chainsToReset = append(chainsToReset, chainID)
		}
	}

	if len(chainsToReset) > 0 {
		return h.ResetMerkle(ctx, chainsToReset...)
	}
	return nil
}

func (h *Hinkal) GetHinkalTreeRootHash(ctx context.Context, chainID int) (*big.Int, error) {
	hinkalAddress := h.HinkalAddress(chainID)
	if hinkalAddress == "" {
		return nil, fmt.Errorf("getHinkalTreeRootHash: chain %d is not synced", chainID)
	}
	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return nil, err
	}

	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := constants.OriginalDeployer(chainID)
		if err != nil {
			return nil, err
		}
		client := solana.NewClient(rpcURL)
		return solana.FetchMerkleTreeRootHash(ctx, client, hinkalAddress, originalDeployer)
	}
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	data, err := parsedABI.Pack("getRootHash")
	if err != nil {
		return nil, err
	}
	address := common.HexToAddress(hinkalAddress)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	results, err := parsedABI.Unpack("getRootHash", out)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("getHinkalTreeRootHash: empty result")
	}
	rootHash, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("getHinkalTreeRootHash: unexpected type %T", results[0])
	}
	return rootHash, nil
}

func (h *Hinkal) storeChainState(chainID int, commitments commitmentsSnapshot, nullifiers nullifierSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.CommitmentsSnapshotServiceByChain[chainID] = commitments
	h.NullifierSnapshotServiceByChain[chainID] = nullifiers
	h.MerkleTreeHinkalByChain[chainID] = commitments.MerkleTree()
	h.EncryptedOutputsByChain[chainID] = commitments.EncryptedOutputs()
	h.NullifiersByChain[chainID] = nullifiers.Nullifiers()
}

func (h *Hinkal) merkleTree(chainID int) merkletree.MerkleTree {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.MerkleTreeHinkalByChain[chainID]
}

func (h *Hinkal) MerkleTree(chainID int) merkletree.MerkleTree {
	return h.merkleTree(chainID)
}

func (h *Hinkal) GetBalances(
	ctx context.Context,
	chainID int,
	passedShieldedPublicKey string,
	ethAddress string,
	resetCacheBefore bool,
	useBlockedUtxos bool,
) (map[string]types.TokenBalance, error) {
	return balance.GetShieldedBalance(
		ctx,
		h,
		chainID,
		passedShieldedPublicKey,
		ethAddress,
		resetCacheBefore,
		h.generateProofRemotely,
		useBlockedUtxos,
	)
}

func (h *Hinkal) GetTotalBalance(
	ctx context.Context,
	chainID int,
	userKeys *cryptokeys.UserKeys,
	ethAddress string,
	resetCacheBefore bool,
	useBlockedUtxos bool,
) ([]types.TokenBalance, error) {
	uk := userKeys
	if uk == nil {
		uk = h.UserKeys
	}
	shieldedPublicKey, err := uk.GetShieldedPublicKey()
	if err != nil {
		return nil, err
	}

	shieldedBalances, err := h.GetBalances(
		ctx,
		chainID,
		shieldedPublicKey,
		ethAddress,
		resetCacheBefore,
		useBlockedUtxos,
	)
	if err != nil {
		return nil, err
	}

	tokenRegistry, err := api.GetTokensForChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	totalBalances := make([]types.TokenBalance, 0, len(tokenRegistry))
	for _, token := range tokenRegistry {
		shieldedBalance, ok := shieldedBalances[strings.ToLower(token.Erc20TokenAddress)]
		tokenBalance := new(big.Int)
		timestamp := "0"
		if ok {
			if shieldedBalance.Balance != nil {
				tokenBalance = shieldedBalance.Balance
			}
			if shieldedBalance.Timestamp != "" {
				timestamp = shieldedBalance.Timestamp
			}
		}
		totalBalances = append(totalBalances, types.TokenBalance{
			Token:     token,
			Balance:   tokenBalance,
			Timestamp: timestamp,
		})
	}
	return totalBalances, nil
}

func (h *Hinkal) GetStuckShieldedBalances(ctx context.Context, chainID int, userKeys *cryptokeys.UserKeys, ethAddress string) ([]types.TokenBalance, error) {
	balances, err := h.GetTotalBalance(ctx, chainID, userKeys, ethAddress, false, true)
	if err != nil {
		return nil, err
	}
	out := make([]types.TokenBalance, 0, len(balances))
	for _, b := range balances {
		if b.Balance.Sign() > 0 {
			out = append(out, b)
		}
	}
	return out, nil
}

func (h *Hinkal) GetRandomRelay(ctx context.Context, chainID int, markAsPending bool) (string, error) {
	return api.GetIdleRelay(ctx, chainID, markAsPending)
}

func (h *Hinkal) GetSupportedChains() []int {
	h.mu.RLock()
	hasEthereum := h.ethereumProviderAdapter != nil
	hasSolana := h.solanaProviderAdapter != nil
	hasTron := h.tronProviderAdapter != nil
	h.mu.RUnlock()

	if hasEthereum && hasSolana {
		return copyInts(constants.WalletSupportedChains)
	}
	if hasEthereum {
		chains := make([]int, 0, len(constants.HinkalSupportedChains))
		for _, chainID := range constants.HinkalSupportedChains {
			if !constants.IsSolanaLike(chainID) && !constants.IsTronLike(chainID) {
				chains = append(chains, chainID)
			}
		}
		return chains
	}
	if hasSolana {
		chains := make([]int, 0, len(constants.HinkalSupportedChains))
		for _, chainID := range constants.HinkalSupportedChains {
			if constants.IsSolanaLike(chainID) {
				chains = append(chains, chainID)
			}
		}
		return chains
	}
	if hasTron {
		return []int{h.tronChainID}
	}
	return nil
}

func copyInts(values []int) []int {
	out := make([]int, len(values))
	copy(out, values)
	return out
}

func (h *Hinkal) GetShieldedPublicKey() (string, error) {
	return h.UserKeys.GetShieldedPublicKey()
}

func (h *Hinkal) AreMerkleTreeUpdatesDisabled() bool {
	return h.disableMerkleTreeUpdates
}

func (h *Hinkal) UpdateMerkleTreeUpdates(value bool) {
	h.disableMerkleTreeUpdates = value
}

func (h *Hinkal) GenerateProofRemotely() bool {
	return h.generateProofRemotely
}

func (h *Hinkal) AllowParallelBalanceLocalDecryption() bool {
	return h.allowParallelBalanceLocalDecryption
}

func (h *Hinkal) BalanceFetchingMutex(chainID int) *sync.Mutex {
	m, _ := h.balanceFetchingMutexByChain.LoadOrStore(chainID, &sync.Mutex{})
	mutex, ok := m.(*sync.Mutex)
	if !ok {
		panic("balanceFetchingMutexByChain holds a non-*sync.Mutex value")
	}
	return mutex
}

func (h *Hinkal) enforceRateLimit(actionKey string, args ...any) error {
	argsKey, err := json.Marshal(args)
	if err != nil {
		return err
	}
	h.lastCallMu.Lock()
	defer h.lastCallMu.Unlock()
	now := time.Now()
	if last, ok := h.lastCallState[actionKey]; ok &&
		last.argsKey == string(argsKey) &&
		now.Sub(last.timestamp) < rateLimitInterval {
		return fmt.Errorf("%s was already called with the same arguments", actionKey)
	}
	h.lastCallState[actionKey] = rateLimitCall{timestamp: now, argsKey: string(argsKey)}
	return nil
}

func (h *Hinkal) GetUserKeys() *cryptokeys.UserKeys {
	return h.UserKeys
}

func (h *Hinkal) EncryptedOutputs(chainID int) []*types.EncryptedOutputWithSign {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.EncryptedOutputsByChain[chainID]
}

func (h *Hinkal) Nullifiers(chainID int) map[string]struct{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.NullifiersByChain[chainID]
}

func (h *Hinkal) CacheDevice() types.ICacheDevice {
	return h.cacheDevice
}

func (h *Hinkal) HinkalAddress(chainID int) string {
	address, err := constants.HinkalAddress(chainID)
	if err != nil {
		return ""
	}
	return address
}
