package pretransaction

import (
	"context"
	"math/big"
	"strings"

	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func MergeWithFeeStructure(
	ctx context.Context,
	chainID int,
	erc20Addresses *[]string,
	amountChanges *[]*big.Int,
	feeStructure types.FeeStructure,
) error {
	feeTokenIndex := -1
	for i, tokenAddress := range *erc20Addresses {
		if strings.EqualFold(tokenAddress, feeStructure.FeeToken) {
			feeTokenIndex = i
			break
		}
	}

	if feeTokenIndex == -1 {
		*erc20Addresses = append(*erc20Addresses, feeStructure.FeeToken)
		*amountChanges = append(*amountChanges, new(big.Int).Neg(feeStructure.FlatFee))
		return nil
	}

	changes := *amountChanges
	if changes[feeTokenIndex].Sign() > 0 && changes[feeTokenIndex].Cmp(feeStructure.FlatFee) < 0 {
		feeUnit, err := web3.GetErc20TokenFromAPI(ctx, chainID, feeStructure.FeeToken)
		if err != nil {
			return err
		}
		return &errorhandling.FeeOverTransactionValueError{
			TotalFeeWEI: feeStructure.FlatFee,
			FeeUnit:     feeUnit,
		}
	}

	changes[feeTokenIndex] = new(big.Int).Sub(changes[feeTokenIndex], feeStructure.FlatFee)
	return nil
}
