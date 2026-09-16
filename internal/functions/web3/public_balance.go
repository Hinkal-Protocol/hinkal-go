package web3

import (
	"context"
	"math/big"
	"strings"

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
