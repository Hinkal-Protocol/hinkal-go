package web3

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type ApprovalRequirement struct {
	TokenAddress   string
	RequiredAmount *big.Int
}

func approveTokenToHinkal(
	ctx context.Context,
	adapter types.IProviderAdapter,
	chainID int,
	spender common.Address,
	amount *big.Int,
	token types.ERC20Token,
) error {
	client, err := adapter.GetFetchClient(chainID)
	if err != nil {
		return err
	}
	ownerStr, err := adapter.GetAddress(ctx)
	if err != nil {
		return err
	}
	owner := common.HexToAddress(ownerStr)
	tokenAddr := common.HexToAddress(token.Erc20TokenAddress)

	allowance, err := Erc20Allowance(ctx, client, tokenAddr, owner, spender)
	if err != nil {
		return err
	}
	if allowance.Cmp(amount) >= 0 {
		return nil
	}

	// USDT on Ethereum mainnet requires the allowance to be reset to 0 before a new approval.
	if constants.GetNonLocalhostChainID(chainID) == constants.ChainIDs.EthMainnet &&
		strings.EqualFold(token.Symbol, "usdt") && allowance.Sign() > 0 {
		resetData, err := Erc20ApproveCalldata(spender, big.NewInt(0))
		if err != nil {
			return err
		}
		resetTx, err := adapter.SendTransaction(ctx, types.TransactionRequest{To: tokenAddr.Hex(), Data: resetData})
		if err != nil {
			return err
		}
		if _, err := adapter.WaitForTransaction(ctx, chainID, resetTx.Hash, 1); err != nil {
			return err
		}
	}

	confirmations := uint64(1)
	if chainID == constants.ChainIDs.Base {
		confirmations = 2
	}

	data, err := Erc20ApproveCalldata(spender, amount)
	if err != nil {
		return err
	}
	tx, err := adapter.SendTransaction(ctx, types.TransactionRequest{To: tokenAddr.Hex(), Data: data})
	if err != nil {
		return err
	}
	_, err = adapter.WaitForTransaction(ctx, chainID, tx.Hash, confirmations)
	return err
}

func ApproveTokensToHinkal(
	ctx context.Context,
	adapter types.IProviderAdapter,
	chainID int,
	spender common.Address,
	tokensToApprove []types.ERC20Token,
	amounts []*big.Int,
) error {
	for i, amount := range amounts {
		token := tokensToApprove[i]
		if amount.Sign() > 0 && token.Erc20TokenAddress != constants.ZeroAddress {
			if err := approveTokenToHinkal(ctx, adapter, chainID, spender, amount, token); err != nil {
				return err
			}
		}
	}
	return nil
}

func WaitForErc20Approvals(
	ctx context.Context,
	adapter types.IProviderAdapter,
	chainID int,
	owner, spender common.Address,
	requirements []ApprovalRequirement,
	maxAttempts int,
	interval time.Duration,
) error {
	valid := make([]ApprovalRequirement, 0, len(requirements))
	for _, r := range requirements {
		if r.RequiredAmount.Sign() > 0 && !strings.EqualFold(r.TokenAddress, constants.ZeroAddress) {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		return nil
	}

	client, err := adapter.GetFetchClient(chainID)
	if err != nil {
		return err
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		allApproved := true
		for _, r := range valid {
			allowance, err := Erc20Allowance(ctx, client, common.HexToAddress(r.TokenAddress), owner, spender)
			if err != nil {
				return err
			}
			if allowance.Cmp(r.RequiredAmount) < 0 {
				allApproved = false
				break
			}
		}
		if allApproved {
			return nil
		}
		time.Sleep(interval)
	}
	return nil
}
