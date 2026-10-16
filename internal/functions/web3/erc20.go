package web3

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetErc20TokenFromAPI(ctx context.Context, chainID int, address string) (*types.ERC20Token, error) {
	tokens, err := api.GetTokensInfo(ctx, chainID, []string{address}, false)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 || tokens[0] == nil {
		return nil, nil
	}
	token := *tokens[0]
	token.ChainID = chainID
	return &token, nil
}

func GetErc20TokensFromAPI(ctx context.Context, chainID int, addresses []string) ([]*types.ERC20Token, error) {
	if len(addresses) == 0 {
		return nil, nil
	}
	tokens, err := api.GetTokensInfo(ctx, chainID, addresses, false)
	if err != nil {
		return nil, err
	}
	for _, token := range tokens {
		if token != nil {
			token.ChainID = chainID
		}
	}
	return tokens, nil
}

func OverlayTokensWithRegistryData(ctx context.Context, tokens []types.ERC20Token, chainID int) ([]types.ERC20Token, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	addresses := make([]string, len(tokens))
	for i, token := range tokens {
		addresses[i] = token.Erc20TokenAddress
	}
	registryTokens, err := GetErc20TokensFromAPI(ctx, chainID, addresses)
	if err != nil {
		return nil, err
	}

	overlaid := make([]types.ERC20Token, len(tokens))
	for index, token := range tokens {
		overlaid[index] = token
		if index >= len(registryTokens) || registryTokens[index] == nil {
			continue
		}
		registryToken := registryTokens[index]
		overlaid[index].Name = registryToken.Name
		overlaid[index].Symbol = registryToken.Symbol
		overlaid[index].LogoURI = registryToken.LogoURI
		overlaid[index].Decimals = registryToken.Decimals
	}
	return overlaid, nil
}

func IsNFTToken(token types.ERC20Token) bool {
	switch constants.TokenType(token.NFTTokenType) {
	case constants.TokenTypeERC721,
		constants.TokenTypeERC1155,
		constants.TokenTypeMetaplexNFT,
		constants.TokenTypeToken2022NFT,
		constants.TokenTypeCompressedNFT:
		return true
	default:
		return false
	}
}

func GetErc20TokensForChainAPI(ctx context.Context, chainID int) ([]types.ERC20Token, error) {
	return api.GetTokensForChain(ctx, chainID)
}

func ResolveERC20Tokens(ctx context.Context, chainID int, erc20Addresses []string) ([]types.ERC20Token, error) {
	if len(erc20Addresses) == 0 {
		return nil, nil
	}
	apiTokens, err := api.GetTokensInfo(ctx, chainID, erc20Addresses, false)
	if err != nil {
		return nil, err
	}
	tokens := make([]types.ERC20Token, len(erc20Addresses))
	for i, address := range erc20Addresses {
		if i < len(apiTokens) && apiTokens[i] != nil {
			tokens[i] = *apiTokens[i]
			tokens[i].ChainID = chainID
			continue
		}
		tokens[i] = types.ERC20Token{ChainID: chainID, Erc20TokenAddress: address}
	}
	return tokens, nil
}

func ResolveERC20TokenStrict(ctx context.Context, chainID int, erc20Address string) (types.ERC20Token, error) {
	tokens, err := ResolveERC20TokensStrict(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.ERC20Token{}, err
	}
	return tokens[0], nil
}

func ResolveERC20TokensStrict(ctx context.Context, chainID int, erc20Addresses []string) ([]types.ERC20Token, error) {
	if len(erc20Addresses) == 0 {
		return nil, nil
	}

	apiTokens, err := api.GetTokensInfo(ctx, chainID, erc20Addresses, false)
	if err != nil {
		return nil, err
	}
	if len(apiTokens) != len(erc20Addresses) {
		return nil, fmt.Errorf("web3: expected %d token records, got %d", len(erc20Addresses), len(apiTokens))
	}

	tokens := make([]types.ERC20Token, len(erc20Addresses))
	for i, token := range apiTokens {
		if token == nil {
			return nil, fmt.Errorf("web3: token not found: %s on chain %d", erc20Addresses[i], chainID)
		}
		tokens[i] = *token
		tokens[i].ChainID = chainID
	}
	return tokens, nil
}

const erc20ABIJSON = `[
  {"name":"allowance","type":"function","stateMutability":"view","inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
  {"name":"approve","type":"function","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},
  {"name":"balanceOf","type":"function","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
  {"name":"transfer","type":"function","stateMutability":"nonpayable","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}
]`

var erc20ABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		panic(fmt.Sprintf("web3: invalid erc20 abi: %v", err))
	}
	return parsed
}()

// Erc20Allowance reads token.allowance(owner, spender) via eth_call.
func Erc20Allowance(ctx context.Context, client *ethclient.Client, token, owner, spender common.Address) (*big.Int, error) {
	data, err := erc20ABI.Pack("allowance", owner, spender)
	if err != nil {
		return nil, err
	}
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	values, err := erc20ABI.Unpack("allowance", out)
	if err != nil {
		return nil, err
	}
	allowance, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("web3: unexpected allowance type %T", values[0])
	}
	return allowance, nil
}

// Erc20ApproveCalldata encodes approve(spender, amount).
func Erc20ApproveCalldata(spender common.Address, amount *big.Int) ([]byte, error) {
	return erc20ABI.Pack("approve", spender, amount)
}

// Erc20TransferCalldata encodes transfer(to, amount).
func Erc20TransferCalldata(to common.Address, amount *big.Int) ([]byte, error) {
	return erc20ABI.Pack("transfer", to, amount)
}

// Erc20BalanceOf reads token.balanceOf(owner) via eth_call.
func Erc20BalanceOf(ctx context.Context, client *ethclient.Client, token, owner common.Address) (*big.Int, error) {
	data, err := erc20ABI.Pack("balanceOf", owner)
	if err != nil {
		return nil, err
	}
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	values, err := erc20ABI.Unpack("balanceOf", out)
	if err != nil {
		return nil, err
	}
	balance, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("web3: unexpected balanceOf type %T", values[0])
	}
	return balance, nil
}
