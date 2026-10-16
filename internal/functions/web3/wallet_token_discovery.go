package web3

import (
	"context"
	"encoding/binary"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func GetSolanaWalletTokenAddresses(ctx context.Context, chainID int, walletAddress string) ([]string, error) {
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, err
	}
	client := api.NewSolanaRPCClientWithFallback(rpcURL)
	vault, err := solana.PublicKeyFromBase58(walletAddress)
	if err != nil {
		return nil, err
	}
	lamports, err := client.GetBalance(ctx, vault, rpc.CommitmentConfirmed)
	if err != nil {
		return nil, err
	}

	tokens := make([]string, 0)
	seen := map[string]bool{}
	add := func(token string) {
		if seen[token] {
			return
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	if lamports.Value > 0 {
		add(constants.SolanaNativeAddress)
	}
	for _, programID := range []solana.PublicKey{solana.TokenProgramID, solana.Token2022ProgramID} {
		accounts, err := client.GetTokenAccountsByOwner(
			ctx,
			vault,
			&rpc.GetTokenAccountsConfig{ProgramId: &programID},
			&rpc.GetTokenAccountsOpts{Commitment: rpc.CommitmentConfirmed, Encoding: solana.EncodingBase64},
		)
		if err != nil {
			return nil, err
		}
		if accounts == nil {
			continue
		}
		for _, account := range accounts.Value {
			if account == nil || account.Account.Data == nil {
				continue
			}
			data := account.Account.Data.GetBinary()
			if len(data) >= 72 && binary.LittleEndian.Uint64(data[64:72]) > 0 {
				add(solana.PublicKeyFromBytes(data[:32]).String())
			}
		}
	}
	return tokens, nil
}

func GetTronWalletTokenAddresses(ctx context.Context, walletAddress string, chainID int) ([]string, error) {
	client, err := tron.NewWalletClient(chainID)
	if err != nil {
		return nil, err
	}
	base58Address, err := utils.ToTronBase58IfHex(walletAddress)
	if err != nil {
		return nil, err
	}
	return client.GetHeldTokenAddresses(ctx, base58Address)
}
