package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func hinkalActionReceive(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	deltaAmountsBase []*big.Int, // should be positive
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	subAccount *types.TemporarySubAccount,
	action types.AdminTransactionType,
) (string, error) {
	if subAccount == nil {
		return "", errors.New("hinkalActionReceive: subAccount should be provided")
	}
	if feeToken == "" {
		return "", errors.New("Fee token is required in hinkalActionReceive")
	}

	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}

	walletAddress, _ := privatewallet.AddressFromSubAccount(chainID, *subAccount)
	if walletAddress == "" {
		return "", errors.New("Public account address is not available")
	}
	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}
	deltaAmounts := pretransaction.ModifyVolatileTokenAmountChanges(ctx, chainID, erc20Tokens, deltaAmountsBase, walletAddress)

	transferOps, err := privatewallet.CreateTransferEmporiumOpsBatch(chainID, erc20Addresses, deltaAmounts, "")
	if err != nil {
		return "", err
	}

	feeStructure := feeStructureOverride
	if feeStructure == nil {
		calls := make([]types.CallInfo, len(transferOps))
		for i, transferOp := range transferOps {
			call, err := privatewallet.ConvertEmporiumOpToCallInfo(transferOp, walletAddress, chainID)
			if err != nil {
				return "", err
			}
			calls[i] = call
		}
		resolved, err := pretransaction.GetFeeStructure(ctx, chainID, feeToken, erc20Addresses, types.ExternalActionEmporium, calls, constants.HinkalShieldVariableRate(), nil)
		if err != nil {
			return "", err
		}
		feeStructure = &resolved
	}

	modifiedFeeStructure := fees.CalculateModifiedFeeStructureForTokens(ctx, chainID, erc20Tokens, deltaAmounts, *feeStructure)

	emporiumTokenChanges := make([]bridging.TokenChange, len(erc20Tokens))
	onChainCreation := make([]bool, len(deltaAmounts))
	for index, token := range erc20Tokens {
		emporiumTokenChanges[index] = bridging.TokenChange{Token: token, Amount: new(big.Int).Neg(deltaAmounts[index])}
	}

	return actionPrivateWallet(
		ctx,
		hinkal,
		chainID,
		erc20Tokens,
		deltaAmounts,
		onChainCreation,
		transferOps,
		emporiumTokenChanges,
		subAccount,
		feeToken,
		&modifiedFeeStructure,
		"",
		action,
		nil,
	)
}
