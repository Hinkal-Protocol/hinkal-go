package tron

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type ApprovalRequirement struct {
	TokenAddress   string
	RequiredAmount *big.Int
}

const tronErc20ABIJSON = `[
  {"name":"allowance","type":"function","stateMutability":"view","inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
  {"name":"approve","type":"function","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}
]`

var tronErc20ABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(tronErc20ABIJSON))
	if err != nil {
		panic(fmt.Sprintf("tron: invalid erc20 abi: %v", err))
	}
	return parsed
}()

// TronErc20ApproveCalldata encodes approve(spender, amount) for a Tron token.
func TronErc20ApproveCalldata(spenderHex common.Address, amount *big.Int) ([]byte, error) {
	return tronErc20ABI.Pack("approve", spenderHex, amount)
}

func tronAllowance(ctx context.Context, chainID int, tokenHex, ownerHex, spenderHex string) (*big.Int, error) {
	data, err := tronErc20ABI.Pack("allowance", common.HexToAddress(ownerHex), common.HexToAddress(spenderHex))
	if err != nil {
		return nil, err
	}
	result, err := ethCall(ctx, chainID, ownerHex, tokenHex, data, nil)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(common.FromHex(result)), nil
}

// WaitForTronErc20Approvals polls token allowances until they meet the requirements. owner/spender
// may be base58 or hex; they are normalized to hex for eth_call.
func WaitForTronErc20Approvals(
	ctx context.Context,
	chainID int,
	owner, spender string,
	requirements []ApprovalRequirement,
	maxAttempts int,
	interval time.Duration,
) error {
	ownerHex, err := utils.AddressToHexFormat(owner)
	if err != nil {
		return err
	}
	spenderHex, err := utils.AddressToHexFormat(spender)
	if err != nil {
		return err
	}

	valid := make([]ApprovalRequirement, 0, len(requirements))
	for _, r := range requirements {
		if r.RequiredAmount.Sign() > 0 && !strings.EqualFold(r.TokenAddress, constants.ZeroAddress) && r.TokenAddress != "" {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		return nil
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		allApproved := true
		for _, r := range valid {
			tokenHex, err := utils.AddressToHexFormat(r.TokenAddress)
			if err != nil {
				return err
			}
			allowance, err := tronAllowance(ctx, chainID, tokenHex, ownerHex, spenderHex)
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
