package transactions

import (
	"context"
	"errors"
	"log"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/addresses"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type chainNonceResult struct {
	chainID      int
	nonceEntries []api.TemporaryWalletNonceEntry
}

type temporaryWalletEntry struct {
	ChainID             int
	Nonce               int
	RecoveryDestination types.TemporaryWalletRecoveryDestination
}

type temporaryWalletAddressEntry struct {
	PrivateKey string
	Entries    []temporaryWalletEntry
}

// temporaryWalletAddressMap = Map<string, temporaryWalletAddressEntry>
type temporaryWalletAddressMap struct {
	ethAddresses []string
	entries      map[string]*temporaryWalletAddressEntry
}

func newTemporaryWalletAddressMap() *temporaryWalletAddressMap {
	return &temporaryWalletAddressMap{entries: map[string]*temporaryWalletAddressEntry{}}
}

func getTemporaryWalletTokenAddresses(ctx context.Context, chainID int, walletAddress string) ([]string, error) {
	if constants.IsTronLike(chainID) {
		return web3.GetTronWalletTokenAddresses(ctx, walletAddress, chainID)
	}
	if constants.IsTempo(chainID) {
		registryTokens, err := web3.GetErc20TokensForChainAPI(ctx, chainID)
		if err != nil {
			return nil, err
		}
		tokenAddresses := make([]string, 0, len(registryTokens))
		for _, token := range registryTokens {
			if !slices.Contains(tokenAddresses, token.Erc20TokenAddress) {
				tokenAddresses = append(tokenAddresses, token.Erc20TokenAddress)
			}
		}
		return tokenAddresses, nil
	}
	if constants.IsSolanaLike(chainID) {
		return web3.GetSolanaWalletTokenAddresses(ctx, chainID, walletAddress)
	}
	return web3.GetEvmWalletTokenAddresses(ctx, chainID, walletAddress)
}

func getTemporaryWalletTokens(ctx context.Context, chainID int, walletAddress string) ([]types.ERC20Token, error) {
	tokenAddresses, err := getTemporaryWalletTokenAddresses(ctx, chainID, walletAddress)
	if err != nil {
		return nil, err
	}
	if len(tokenAddresses) == 0 {
		return []types.ERC20Token{}, nil
	}

	tokensFromBackend, err := web3.GetErc20TokensFromAPI(ctx, chainID, tokenAddresses)
	if err != nil {
		return nil, err
	}
	erc20Tokens := make([]types.ERC20Token, 0, len(tokensFromBackend))
	for _, token := range tokensFromBackend {
		if token != nil && !token.IsSpam {
			erc20Tokens = append(erc20Tokens, *token)
		}
	}

	overlaidTokens, err := web3.OverlayTokensWithRegistryData(ctx, erc20Tokens, chainID)
	if err != nil {
		return nil, err
	}
	tokens := make([]types.ERC20Token, 0, len(overlaidTokens))
	for _, token := range overlaidTokens {
		if !web3.IsNFTToken(token) {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func refreshPublicBalancesForChain(
	ctx context.Context,
	ethereumAddress string,
	currentChainID int,
	tokensToUpdate []types.ERC20Token,
) ([]types.TokenBalance, error) {
	if constants.IsSolanaLike(currentChainID) {
		return web3.GetSolanaPublicBalances(ctx, currentChainID, ethereumAddress, tokensToUpdate)
	}
	if currentChainID == constants.ChainIDs.HyperCore {
		return web3.GetHyperCorePublicBalances(ctx, currentChainID, ethereumAddress, tokensToUpdate)
	}

	hexAddress, err := utils.AddressToHexFormat(ethereumAddress)
	if err != nil {
		return nil, err
	}
	return web3.GetPublicBalancesOfTokens(ctx, currentChainID, hexAddress, tokensToUpdate)
}

func getBalancesWithUSD(
	ctx context.Context,
	newBalances []types.TokenBalance,
	currentChainID int,
) ([]types.TokenBalanceWithUsdValue, error) {
	tokenAddresses := make([]string, len(newBalances))
	for i, balance := range newBalances {
		tokenAddresses[i] = balance.Token.Erc20TokenAddress
	}
	prices, err := api.GetTokenPrices(ctx, currentChainID, tokenAddresses)
	if err != nil {
		return nil, err
	}

	balancesWithUSD := make([]types.TokenBalanceWithUsdValue, len(newBalances))
	for index, balance := range newBalances {
		price := 0.0
		if index < len(prices.Prices) {
			price = prices.Prices[index]
		}
		amountInToken, _ := strconv.ParseFloat(web3.GetAmountInToken(balance.Token, balance.Balance), 64)
		balancesWithUSD[index] = types.TokenBalanceWithUsdValue{
			Token:    balance.Token,
			Balance:  balance.Balance,
			UsdValue: amountInToken * price,
		}
	}
	return balancesWithUSD, nil
}

func getTemporaryWalletTokenBalances(ctx context.Context, ethereumAddress string, currentChainID int) ([]types.TokenBalanceWithUsdValue, error) {
	tokensToUpdate, err := getTemporaryWalletTokens(ctx, currentChainID, ethereumAddress)
	if err != nil {
		return nil, err
	}
	newBalances, err := refreshPublicBalancesForChain(ctx, ethereumAddress, currentChainID, tokensToUpdate)
	if err != nil {
		return nil, err
	}
	return getBalancesWithUSD(ctx, newBalances, currentChainID)
}

func isTemporaryWalletInTransit(createdAt string) bool {
	if createdAt == "" {
		return false
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return true
	}
	return time.Since(created).Milliseconds() <= constants.TemporaryWalletInTransitMS
}

func fetchNonceResults(ctx context.Context, hinkal ihinkal.HinkalInternal, chainIDs []int) []chainNonceResult {
	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		log.Printf("Error fetching temporary wallet balances: %v", err)
		return []chainNonceResult{}
	}
	hashedAddress := utils.HashEthereumAddress(ethereumAddress)

	results := make([]chainNonceResult, len(chainIDs))
	group, groupCtx := errgroup.WithContext(ctx)
	for index, chainID := range chainIDs {
		index, chainID := index, chainID
		group.Go(func() error {
			response, err := api.GetTemporaryWalletNonces(groupCtx, chainID, hashedAddress)
			if err != nil {
				return err
			}
			nonceEntries := make([]api.TemporaryWalletNonceEntry, 0, len(response.NonceEntries))
			for _, entry := range response.NonceEntries {
				if !isTemporaryWalletInTransit(entry.CreatedAt) {
					nonceEntries = append(nonceEntries, entry)
				}
			}
			results[index] = chainNonceResult{chainID: chainID, nonceEntries: nonceEntries}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		log.Printf("Error fetching temporary wallet balances: %v", err)
		return []chainNonceResult{}
	}
	return results
}

func buildAddressMap(hinkal ihinkal.HinkalInternal, nonceResults []chainNonceResult) *temporaryWalletAddressMap {
	addressMap := newTemporaryWalletAddressMap()
	for _, result := range nonceResults {
		for _, entry := range result.nonceEntries {
			wallet, err := privatewallet.DeriveTempWallet(hinkal.GetUserKeys(), result.chainID, big.NewInt(int64(entry.Nonce)), entry.IsLegacy)
			if err != nil {
				log.Printf("Skipping invalid temporary wallet nonce %d on chain %d: %v", entry.Nonce, result.chainID, err)
				continue
			}
			walletEntry := temporaryWalletEntry{ChainID: result.chainID, Nonce: entry.Nonce, RecoveryDestination: entry.RecoveryDestination}
			if existing, ok := addressMap.entries[wallet.Address]; ok {
				existing.Entries = append(existing.Entries, walletEntry)
			} else {
				addressMap.ethAddresses = append(addressMap.ethAddresses, wallet.Address)
				addressMap.entries[wallet.Address] = &temporaryWalletAddressEntry{
					PrivateKey: wallet.PrivateKey,
					Entries:    []temporaryWalletEntry{walletEntry},
				}
			}
		}
	}
	return addressMap
}

func getTemporaryWalletAddressMap(ctx context.Context, hinkal ihinkal.HinkalInternal, chainIDs []int) *temporaryWalletAddressMap {
	return buildAddressMap(hinkal, fetchNonceResults(ctx, hinkal, chainIDs))
}

func getTemporaryWalletAddressMapBalances(
	ctx context.Context,
	addressMap *temporaryWalletAddressMap,
	getTokenBalances func(ctx context.Context, ethAddress string, chainID int) ([]types.TokenBalanceWithUsdValue, error),
) []types.TemporaryWalletBalance {
	if getTokenBalances == nil {
		getTokenBalances = getTemporaryWalletTokenBalances
	}

	temporaryWalletBalances := make([][]types.TemporaryWalletBalance, len(addressMap.ethAddresses))
	group := errgroup.Group{}
	for addressIndex, ethAddress := range addressMap.ethAddresses {
		addressIndex, ethAddress := addressIndex, ethAddress
		entry := addressMap.entries[ethAddress]
		group.Go(func() error {
			chainIDsOverride := make([]int, 0, len(entry.Entries))
			for _, item := range entry.Entries {
				if !slices.Contains(chainIDsOverride, item.ChainID) {
					chainIDsOverride = append(chainIDsOverride, item.ChainID)
				}
			}

			chainBalances := make([][]types.TokenBalanceWithUsdValue, len(chainIDsOverride))
			chainGroup := errgroup.Group{}
			for chainIndex, chainID := range chainIDsOverride {
				chainIndex, chainID := chainIndex, chainID
				chainGroup.Go(func() error {
					balances, err := getTokenBalances(ctx, ethAddress, chainID)
					if err != nil {
						log.Printf("Error fetching public balances for chain %d: %v", chainID, err)
						balances = []types.TokenBalanceWithUsdValue{}
					}
					chainBalances[chainIndex] = balances
					return nil
				})
			}
			_ = chainGroup.Wait()
			balancesByChain := make(map[int][]types.TokenBalanceWithUsdValue, len(chainIDsOverride))
			for chainIndex, chainID := range chainIDsOverride {
				balancesByChain[chainID] = chainBalances[chainIndex]
			}

			balances := make([]types.TemporaryWalletBalance, 0)
			for _, item := range entry.Entries {
				for _, balance := range balancesByChain[item.ChainID] {
					if balance.Balance.Sign() <= 0 {
						continue
					}
					balances = append(balances, types.TemporaryWalletBalance{
						TokenBalanceWithUsdValue: balance,
						Nonce:                    item.Nonce,
						EthAddress:               ethAddress,
						PrivateKey:               entry.PrivateKey,
						RecoveryDestination:      item.RecoveryDestination,
					})
				}
			}
			temporaryWalletBalances[addressIndex] = balances
			return nil
		})
	}
	_ = group.Wait()

	flattened := make([]types.TemporaryWalletBalance, 0)
	for _, balances := range temporaryWalletBalances {
		flattened = append(flattened, balances...)
	}
	return flattened
}

func getTemporaryWalletBalances(ctx context.Context, hinkal ihinkal.HinkalInternal, chainIDs []int) []types.TemporaryWalletBalance {
	return getTemporaryWalletAddressMapBalances(ctx, getTemporaryWalletAddressMap(ctx, hinkal, chainIDs), nil)
}

func getTemporaryWalletWithdrawCalls(chainID int, token types.ERC20Token, ethAddress string) ([]types.CallInfo, error) {
	if constants.IsSolanaLike(chainID) {
		return []types.CallInfo{}, nil
	}

	hinkalContractAddress, err := constants.HinkalAddress(chainID)
	if err != nil || hinkalContractAddress == "" {
		return nil, errors.New("Hinkal contract not found")
	}

	if token.Erc20TokenAddress == constants.ZeroAddress {
		return []types.CallInfo{{From: ethAddress, To: hinkalContractAddress, Calldata: "0x", Value: big.NewInt(1)}}, nil
	}

	calldata, err := web3.Erc20TransferCalldata(common.HexToAddress(hinkalContractAddress), big.NewInt(1))
	if err != nil {
		return nil, errors.New("ERC20 interface not found")
	}
	return []types.CallInfo{{
		From:     ethAddress,
		To:       token.Erc20TokenAddress,
		Value:    big.NewInt(0),
		Calldata: "0x" + common.Bytes2Hex(calldata),
	}}, nil
}

func recoverTemporaryWalletBalance(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	balance types.TemporaryWalletBalance,
	recipientAddressOverride string,
) error {
	token, amount, privateKey, ethAddress, recoveryDestination :=
		balance.Token, balance.Balance, balance.PrivateKey, balance.EthAddress, balance.RecoveryDestination

	chainID, err := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
	if err != nil {
		return err
	}
	shieldEligible := recoveryDestination == types.TemporaryWalletRecoveryDestinationConfidential
	canShieldAutomatically := shieldEligible && slices.Contains(hinkal.GetSupportedChains(), chainID)

	isRecipientAddressValid := func(recipientAddress string) bool {
		trimmed := strings.TrimSpace(recipientAddress)
		return trimmed != "" && !addresses.IsRecipientAddressInvalid(trimmed, constants.IsSolanaLike(chainID), constants.IsTronLike(chainID))
	}

	isSolanaRecovery := constants.IsSolanaLike(chainID)
	enteredAddress := strings.TrimSpace(recipientAddressOverride)
	recipientAddress := enteredAddress
	if recipientAddress == "" {
		recipientAddress, err = hinkal.GetEthereumAddress(ctx)
		if err != nil {
			return err
		}
	}
	if !canShieldAutomatically && !isRecipientAddressValid(recipientAddress) {
		return errors.New("Enter a valid destination address for this recovery chain")
	}

	subAccount := types.TemporarySubAccount{}
	if isSolanaRecovery {
		subAccount.SolanaPrivateKey = privateKey
		subAccount.SolanaAddress = ethAddress
	} else {
		subAccount.PrivateKey = privateKey
		subAccount.EthAddress = ethAddress
	}

	if isSolanaRecovery {
		return recoverSolanaBalance(ctx, hinkal, chainID, token, amount, subAccount, shieldEligible, enteredAddress)
	}
	withdrawCalls, err := getTemporaryWalletWithdrawCalls(chainID, token, ethAddress)
	if err != nil {
		return err
	}
	return recoverEvmBalance(
		ctx,
		hinkal,
		chainID,
		token,
		amount,
		subAccount,
		recipientAddress,
		shieldEligible,
		enteredAddress,
		withdrawCalls,
	)
}
