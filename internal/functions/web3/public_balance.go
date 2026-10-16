package web3

import (
	"context"
	"math/big"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/ethereum/go-ethereum/common"
	solanago "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetPublicBalanceByAddress(ctx context.Context, chainID int, token types.ERC20Token, address string) (*big.Int, error) {
	tokenAddress := token.Erc20TokenAddress
	if constants.IsSolanaLike(chainID) {
		return getSolanaPublicBalanceByAddress(ctx, chainID, token, address)
	}
	client, err := fetchClient(chainID)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(tokenAddress, constants.ZeroAddress) {
		return client.BalanceAt(ctx, common.HexToAddress(address), nil)
	}
	return Erc20BalanceOf(ctx, client, common.HexToAddress(tokenAddress), common.HexToAddress(address))
}

func getSolanaPublicBalanceByAddress(ctx context.Context, chainID int, token types.ERC20Token, address string) (*big.Int, error) {
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, err
	}
	client := api.NewSolanaClientWithFallback(rpcURL)
	if strings.EqualFold(token.Erc20TokenAddress, constants.SolanaNativeAddress) {
		return client.GetBalance(ctx, address)
	}

	ata, err := SolanaAtaAddress(token, address)
	if err != nil {
		return nil, err
	}
	return client.GetTokenBalanceByAta(ctx, ata)
}

// SolanaAtaAddress derives the associated token account holding this token for an owner. The token
// program is part of the derivation, so an SPL token and a Token-2022 token with the same mint and
// owner land on different addresses.
func SolanaAtaAddress(token types.ERC20Token, owner string) (string, error) {
	ownerKey, err := solanago.PublicKeyFromBase58(owner)
	if err != nil {
		return "", err
	}
	mint, err := solanago.PublicKeyFromBase58(token.Erc20TokenAddress)
	if err != nil {
		return "", err
	}
	tokenProgram := solanago.TokenProgramID
	if token.Is2022Program {
		tokenProgram = solanago.Token2022ProgramID
	}
	ata, _, err := solanago.FindAssociatedTokenAddressWithProgram(ownerKey, mint, tokenProgram)
	if err != nil {
		return "", err
	}
	return ata.String(), nil
}

func getPublicBalance(ctx context.Context, chainID int, ethereumAddress string, token types.ERC20Token) *types.TokenBalance {
	balance, err := GetPublicBalanceByAddress(ctx, chainID, token, ethereumAddress)
	if err != nil || balance == nil {
		return nil
	}
	if balance.Sign() <= 0 {
		return nil
	}
	return &types.TokenBalance{Token: token, Balance: balance}
}

func GetPublicBalancesOfTokens(ctx context.Context, chainID int, ethereumAddress string, tokens []types.ERC20Token) ([]types.TokenBalance, error) {
	if tokens == nil {
		chainTokens, err := GetErc20TokensForChainAPI(ctx, chainID)
		if err != nil {
			return nil, err
		}
		tokens = chainTokens
	}
	tokensList := make([]types.ERC20Token, 0, len(tokens))
	for _, token := range tokens {
		if !token.Is2022Program {
			tokensList = append(tokensList, token)
		}
	}

	balances := make([]*types.TokenBalance, len(tokensList))
	group, groupCtx := errgroup.WithContext(ctx)
	for index, token := range tokensList {
		index, token := index, token
		group.Go(func() error {
			balances[index] = getPublicBalance(groupCtx, chainID, ethereumAddress, token)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	result := make([]types.TokenBalance, 0, len(balances))
	for _, balance := range balances {
		if balance != nil {
			result = append(result, *balance)
		}
	}
	return result, nil
}

func GetHyperCorePublicBalances(ctx context.Context, chainID int, ethereumAddress string, tokensToUpdate []types.ERC20Token) ([]types.TokenBalance, error) {
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, err
	}
	infoURL := rpcURL + "/info"
	var spotState types.HyperCoreSpotState
	var perpsState types.HyperCorePerpsState
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return api.Post(groupCtx, infoURL, map[string]string{"type": "spotClearinghouseState", "user": ethereumAddress}, &spotState)
	})
	group.Go(func() error {
		return api.Post(groupCtx, infoURL, map[string]string{"type": "clearinghouseState", "user": ethereumAddress}, &perpsState)
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	tokenBalances := make([]types.TokenBalance, 0, len(tokensToUpdate))
	for _, token := range tokensToUpdate {
		amount := ""
		if strings.EqualFold(token.Erc20TokenAddress, constants.HyperCorePerpsUSDCAddress) {
			if perpsState.MarginSummary != nil {
				amount = perpsState.MarginSummary.AccountValue
			}
		} else {
			for _, balance := range spotState.Balances {
				if balance.Coin == token.Symbol {
					amount = balance.Total
					break
				}
			}
		}
		balance := big.NewInt(0)
		if amount != "" {
			balance, err = GetAmountInWei(token, amount)
			if err != nil {
				return nil, err
			}
		}
		if balance.Sign() > 0 {
			tokenBalances = append(tokenBalances, types.TokenBalance{Token: token, Balance: balance})
		}
	}
	return tokenBalances, nil
}

func GetSolanaPublicBalances(ctx context.Context, chainID int, ethereumAddress string, tokensToUpdate []types.ERC20Token) ([]types.TokenBalance, error) {
	tokenBalances := make([]types.TokenBalance, len(tokensToUpdate))
	group, groupCtx := errgroup.WithContext(ctx)
	for index, token := range tokensToUpdate {
		index, token := index, token
		group.Go(func() error {
			if IsNFTToken(token) {
				tokenBalances[index] = types.TokenBalance{Token: token, Balance: big.NewInt(1)}
				return nil
			}
			balance, err := getSolanaPublicBalanceByAddress(groupCtx, chainID, token, ethereumAddress)
			if err != nil {
				return err
			}
			tokenBalances[index] = types.TokenBalance{Token: token, Balance: balance}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	result := make([]types.TokenBalance, 0, len(tokenBalances))
	for _, tokenBalance := range tokenBalances {
		if tokenBalance.Balance.Sign() > 0 {
			result = append(result, tokenBalance)
		}
	}
	return result, nil
}
