package transactions

import (
	"context"
	"math/big"
	"strings"

	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

// resolveFeeTokenInfo finds the full token for the fee address among the tokens this transaction
// already carries, so reading its balance knows the Solana token program. Only a fee token that
// appears nowhere in the transaction needs a lookup.
func resolveFeeTokenInfo(
	ctx context.Context,
	chainID int,
	feeTokenAddress string,
	erc20Tokens []types.ERC20Token,
	emporiumTokenChanges []bridging.TokenChange,
) (types.ERC20Token, error) {
	for _, token := range erc20Tokens {
		if strings.EqualFold(token.Erc20TokenAddress, feeTokenAddress) {
			return token, nil
		}
	}
	for _, change := range emporiumTokenChanges {
		if strings.EqualFold(change.Token.Erc20TokenAddress, feeTokenAddress) {
			return change.Token, nil
		}
	}

	resolved, err := web3.ResolveERC20Tokens(ctx, chainID, []string{feeTokenAddress})
	if err != nil {
		return types.ERC20Token{}, err
	}
	if len(resolved) == 0 {
		return types.ERC20Token{Erc20TokenAddress: feeTokenAddress}, nil
	}
	return resolved[0], nil
}

func mergeWithFeeStructureEmporium(
	ctx context.Context,
	chainID int,
	walletAddress string,
	ops *[]string,
	erc20Addresses *[]string,
	amountChanges *[]*big.Int,
	feeStructure types.FeeStructure,
	feeToken types.ERC20Token,
	feeTokenChange *big.Int,
) error {
	if feeTokenChange == nil {
		feeTokenChange = big.NewInt(0)
	}
	nonPositiveFeeTokenChange := big.NewInt(0)
	if feeTokenChange.Sign() < 0 {
		nonPositiveFeeTokenChange = feeTokenChange
	}

	balance, err := web3.GetPublicBalanceByAddress(ctx, chainID, feeToken, walletAddress)
	if err != nil {
		return err
	}
	netAmount := new(big.Int).Add(balance, nonPositiveFeeTokenChange)

	var feeAmountToTransfer *big.Int
	if netAmount.Sign() < 0 {
		feeAmountToTransfer = feeStructure.FlatFee
	} else {
		feeAmountToTransfer = utils.BigintMax(new(big.Int).Sub(feeStructure.FlatFee, netAmount), big.NewInt(0))
	}
	if feeAmountToTransfer.Sign() == 0 {
		return nil
	}

	toppedUpFeeStructure := feeStructure
	toppedUpFeeStructure.FlatFee = feeAmountToTransfer
	if err := pretransaction.MergeWithFeeStructure(ctx, chainID, erc20Addresses, amountChanges, toppedUpFeeStructure); err != nil {
		return err
	}

	op, err := privatewallet.CreateTransferToEmporiumOp(feeStructure.FeeToken, walletAddress, feeAmountToTransfer, false)
	if err != nil {
		return err
	}
	*ops = append(*ops, op)
	return nil
}
