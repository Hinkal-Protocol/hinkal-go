package balance

import (
	"context"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetShieldedBalance(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	passedShieldedPublicKey string,
	ethAddress string,
	resetCacheBefore bool,
	allowRemoteDecryption bool,
	useBlockedUtxos bool,
) (map[string]types.TokenBalance, error) {
	// Serialize balance refreshing per chain for this user. The Hinkal instance belongs to a
	// single user, so concurrent refreshes for different users don't block each other.
	mutex := hinkal.BalanceFetchingMutex(chainID)
	mutex.Lock()
	defer mutex.Unlock()

	chainBalanceMutex := utils.GetChainBalanceFetchingMutex(chainID)
	chainBalanceMutex.RLock()
	defer chainBalanceMutex.RUnlock()

	if allowRemoteDecryption {
		return getShieldedBalanceRemote(ctx, hinkal, chainID, ethAddress, useBlockedUtxos)
	}

	params := InputUtxoParams{
		Hinkal:                  hinkal,
		ChainID:                 chainID,
		PassedShieldedPublicKey: passedShieldedPublicKey,
		EthAddress:              ethAddress,
		ResetCacheBefore:        resetCacheBefore,
		AllowRemoteDecryption:   allowRemoteDecryption,
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

	tokenRegistry, err := api.GetTokensForChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	balancesMap := make(map[string]types.TokenBalance, len(tokenRegistry))
	for _, token := range tokenRegistry {
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
		balancesMap[strings.ToLower(token.Erc20TokenAddress)] = types.TokenBalance{
			Token:     token,
			Balance:   balance,
			Timestamp: timestamp,
		}
	}

	return balancesMap, nil
}

func getShieldedBalanceRemote(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	ethAddress string,
	useBlockedUtxos bool,
) (map[string]types.TokenBalance, error) {
	hashedEthereumAddress := ""
	if useBlockedUtxos {
		addr := ethAddress
		if constants.IsTronLike(chainID) {
			hexAddr, err := utils.AddressToHexFormat(ethAddress)
			if err != nil {
				return nil, err
			}
			addr = hexAddr
		}
		hashedEthereumAddress = utils.HashEthereumAddress(addr)
	}

	remoteBalances, err := enclave.GetRemoteManagedTokenBalances(ctx, chainID, hinkal.GetUserKeys(), useBlockedUtxos, hashedEthereumAddress)
	if err != nil {
		return nil, err
	}

	tokenRegistry, err := api.GetTokensForChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	balancesMap := make(map[string]types.TokenBalance, len(tokenRegistry))
	for _, token := range tokenRegistry {
		key := strings.ToLower(token.Erc20TokenAddress)
		balance := remoteBalances[key]
		if balance == nil {
			balance = new(big.Int)
		}
		balancesMap[key] = types.TokenBalance{Token: token, Balance: balance}
	}
	return balancesMap, nil
}
