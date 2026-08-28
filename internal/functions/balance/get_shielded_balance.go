package balance

import (
	"context"
	"log"
	"math/big"
	"strings"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetShieldedBalances(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	ethAddressByChain map[int]string,
	passedShieldedPublicKey string,
	resetCacheBefore bool,
	allowRemoteDecryption bool,
	useBlockedUtxos bool,
) (map[int][]types.TokenBalance, error) {
	if allowRemoteDecryption {
		return getRemoteShieldedBalances(ctx, hinkal, ethAddressByChain, useBlockedUtxos)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	balancesByChain := make(map[int][]types.TokenBalance, len(ethAddressByChain))
	for chainID, ethAddress := range ethAddressByChain {
		wg.Go(func() {
			balances, err := runWithChainBalanceLocks(hinkal, chainID, func() ([]types.TokenBalance, error) {
				return getLocalShieldedBalance(ctx, hinkal, chainID, passedShieldedPublicKey, ethAddress, resetCacheBefore, useBlockedUtxos)
			})
			if err != nil {
				log.Printf("error fetching shielded balance for chainId %d: %v", chainID, err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			balancesByChain[chainID] = balances
		})
	}
	wg.Wait()

	return balancesByChain, nil
}

// The first mutex serializes balance refreshing per chain for this user. The Hinkal instance
// belongs to a single user, so concurrent refreshes for different users don't block each other.
// The second one keeps a refresh from running while the blockchain event emitter retrieves events.
func runWithChainBalanceLocks(
	hinkal ihinkal.HinkalInternal,
	chainID int,
	run func() ([]types.TokenBalance, error),
) ([]types.TokenBalance, error) {
	mutex := hinkal.BalanceFetchingMutex(chainID)
	mutex.Lock()
	defer mutex.Unlock()

	chainBalanceMutex := utils.GetChainBalanceFetchingMutex(chainID)
	chainBalanceMutex.RLock()
	defer chainBalanceMutex.RUnlock()

	return run()
}

func getRemoteShieldedBalances(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	ethAddressByChain map[int]string,
	useBlockedUtxos bool,
) (map[int][]types.TokenBalance, error) {
	chainIDsByHashedAddress := make(map[string][]int)
	for chainID, ethAddress := range ethAddressByChain {
		hashedEthereumAddress := ""
		if useBlockedUtxos {
			hashed, err := hashOwnerAddressForChain(chainID, ethAddress)
			if err != nil {
				log.Printf("error hashing owner address for chainId %d: %v", chainID, err)
				continue
			}
			hashedEthereumAddress = hashed
		}
		chainIDsByHashedAddress[hashedEthereumAddress] = append(chainIDsByHashedAddress[hashedEthereumAddress], chainID)
	}

	remoteBalancesByChain := make(map[int]map[string]*big.Int, len(ethAddressByChain))
	for hashedEthereumAddress, chainIDs := range chainIDsByHashedAddress {
		batch, err := enclave.GetRemoteManagedTokenBalances(ctx, chainIDs, hinkal.GetUserKeys(), useBlockedUtxos, hashedEthereumAddress)
		if err != nil {
			log.Printf("error fetching remote shielded balances for chainIds %v: %v", chainIDs, err)
			continue
		}
		for chainID, balances := range batch {
			remoteBalancesByChain[chainID] = balances
		}
	}

	balancesByChain := make(map[int][]types.TokenBalance, len(remoteBalancesByChain))
	for chainID, remoteBalances := range remoteBalancesByChain {
		balances, err := runWithChainBalanceLocks(hinkal, chainID, func() ([]types.TokenBalance, error) {
			return buildTokenBalances(ctx, chainID, func(token types.ERC20Token) (*big.Int, string) {
				balance := remoteBalances[strings.ToLower(token.Erc20TokenAddress)]
				if balance == nil {
					balance = new(big.Int)
				}
				return balance, ""
			})
		})
		if err != nil {
			log.Printf("error fetching shielded balance for chainId %d: %v", chainID, err)
			continue
		}
		balancesByChain[chainID] = balances
	}

	return balancesByChain, nil
}

// Tron stores the hex form of the address, EVM the address as connected (already hex).
// Solana pubkeys are base58 and must stay untouched.
func hashOwnerAddressForChain(chainID int, ethAddress string) (string, error) {
	if constants.IsSolanaLike(chainID) {
		return utils.HashEthereumAddress(ethAddress), nil
	}
	hexAddr, err := utils.AddressToHexFormat(ethAddress)
	if err != nil {
		return "", err
	}
	return utils.HashEthereumAddress(hexAddr), nil
}

func getLocalShieldedBalance(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	passedShieldedPublicKey string,
	ethAddress string,
	resetCacheBefore bool,
	useBlockedUtxos bool,
) ([]types.TokenBalance, error) {
	params := InputUtxoParams{
		Hinkal:                  hinkal,
		ChainID:                 chainID,
		PassedShieldedPublicKey: passedShieldedPublicKey,
		EthAddress:              ethAddress,
		ResetCacheBefore:        resetCacheBefore,
		AllowRemoteDecryption:   false,
	}

	var inputUtxos []*utxo.Utxo
	var err error
	if useBlockedUtxos {
		inputUtxos, err = GetInputUtxoAndBalanceOfStuckUtxos(ctx, params)
	} else {
		inputUtxos, err = GetInputUtxoAndBalance(ctx, params)
	}
	if err != nil {
		return nil, err
	}

	return buildTokenBalances(ctx, chainID, func(token types.ERC20Token) (*big.Int, string) {
		balance := new(big.Int)
		timestamp := ""
		for _, u := range inputUtxos {
			tokenAddress, err := u.GetTokenAddress(chainID)
			if err != nil {
				continue
			}
			if !strings.EqualFold(token.Erc20TokenAddress, tokenAddress) {
				continue
			}
			balance.Add(balance, u.Amount)
			if timestamp == "" {
				timestamp = u.TimeStamp
			}
		}
		return balance, timestamp
	})
}

func buildTokenBalances(
	ctx context.Context,
	chainID int,
	balanceForToken func(token types.ERC20Token) (*big.Int, string),
) ([]types.TokenBalance, error) {
	tokenRegistry, err := api.GetTokensForChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	balances := make([]types.TokenBalance, 0, len(tokenRegistry))
	for _, token := range tokenRegistry {
		balance, timestamp := balanceForToken(token)
		balances = append(balances, types.TokenBalance{
			Token:     token,
			Balance:   balance,
			Timestamp: timestamp,
		})
	}
	return balances, nil
}
