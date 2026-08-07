package tron

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type TokenApprovalAmount struct {
	TokenBase58 string
	TokenHex    string
	Amount      *big.Int
}

func positiveAmount(amount *big.Int) *big.Int {
	if amount != nil && amount.Sign() > 0 {
		return amount
	}
	return big.NewInt(0)
}

func SplitTokensByNative(tokens []types.ERC20Token, amounts []*big.Int) (*big.Int, []TokenApprovalAmount, error) {
	if len(tokens) != len(amounts) {
		return nil, nil, errTokenAmountMismatch
	}

	totalTrxValue := big.NewInt(0)
	approvals := make([]TokenApprovalAmount, 0, len(tokens))
	approvalIndexByToken := make(map[string]int, len(tokens))

	for i, token := range tokens {
		amount := amounts[i]
		if strings.EqualFold(token.Erc20TokenAddress, constants.ZeroAddress) {
			totalTrxValue.Add(totalTrxValue, amount)
			continue
		}

		tokenHex, err := utils.AddressToHexFormat(token.Erc20TokenAddress)
		if err != nil {
			return nil, nil, err
		}
		key := strings.ToLower(tokenHex)
		approvalIndex, ok := approvalIndexByToken[key]
		if !ok {
			tokenBase58, err := utils.EVMHexToTronBase58Address(tokenHex)
			if err != nil {
				return nil, nil, err
			}
			approvalIndex = len(approvals)
			approvalIndexByToken[key] = approvalIndex
			approvals = append(approvals, TokenApprovalAmount{
				TokenBase58: tokenBase58,
				TokenHex:    tokenHex,
				Amount:      big.NewInt(0),
			})
		}
		approvals[approvalIndex].Amount.Add(approvals[approvalIndex].Amount, amount)
	}

	return positiveAmount(totalTrxValue), approvals, nil
}

func ApproveTokens(
	ctx context.Context,
	client TronClient,
	chainID int,
	ownerBase58 string,
	ownerHex string,
	spenderBase58 string,
	approvals []TokenApprovalAmount,
) error {
	if len(approvals) == 0 {
		return nil
	}

	spenderHex, err := utils.AddressToHexFormat(spenderBase58)
	if err != nil {
		return err
	}

	requirements := make([]ApprovalRequirement, 0, len(approvals))
	for _, approval := range approvals {
		if approval.Amount.Sign() <= 0 {
			continue
		}

		allowance, err := tronAllowance(ctx, chainID, approval.TokenHex, ownerHex, spenderHex)
		if err != nil {
			return err
		}
		if allowance.Cmp(approval.Amount) >= 0 {
			continue
		}

		approveData, err := TronErc20ApproveCalldata(common.HexToAddress(spenderHex), approval.Amount)
		if err != nil {
			return err
		}
		approveTx, err := client.WalletClient().TriggerSmartContract(
			ctx,
			ownerBase58,
			approval.TokenBase58,
			approveData,
			0,
			constants.TronDefaultFeeLimitSun,
		)
		if err != nil {
			return fmt.Errorf("tron build approve: %w", err)
		}
		if _, err := client.SignAndBroadcast(ctx, approveTx); err != nil {
			return fmt.Errorf("tron token approval failed: %w", err)
		}
		requirements = append(requirements, ApprovalRequirement{
			TokenAddress:   approval.TokenBase58,
			RequiredAmount: approval.Amount,
		})
	}

	if len(requirements) == 0 {
		return nil
	}
	return WaitForTronErc20Approvals(ctx, chainID, ownerBase58, spenderBase58, requirements, 30, time.Second)
}

func NormalizedTokenAddresses(tokens []types.ERC20Token) ([]string, error) {
	addresses := make([]string, len(tokens))
	for i, token := range tokens {
		if strings.EqualFold(token.Erc20TokenAddress, constants.ZeroAddress) {
			addresses[i] = constants.ZeroAddress
			continue
		}
		tokenHex, err := utils.AddressToHexFormat(token.Erc20TokenAddress)
		if err != nil {
			return nil, err
		}
		addresses[i] = tokenHex
	}
	return addresses, nil
}
