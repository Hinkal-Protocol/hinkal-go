package transactions

import (
	"context"
	"math/big"

	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func mergeWithFeeStructureEmporium(
	ctx context.Context,
	chainID int,
	walletAddress string,
	ops *[]string,
	erc20Addresses *[]string,
	amountChanges *[]*big.Int,
	feeStructure types.FeeStructure,
	feeTokenChange *big.Int,
) error {
	if feeTokenChange == nil {
		feeTokenChange = big.NewInt(0)
	}
	nonPositiveFeeTokenChange := big.NewInt(0)
	if feeTokenChange.Sign() < 0 {
		nonPositiveFeeTokenChange = feeTokenChange
	}

	balance, err := web3.GetPublicBalanceByAddress(ctx, chainID, feeStructure.FeeToken, walletAddress)
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
