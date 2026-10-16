package transactions

import (
	"context"
	"math/big"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func swapOnChainCreation(length int) []bool {
	pattern := []bool{false, true, false}
	if length > len(pattern) {
		length = len(pattern)
	}
	return pattern[:length]
}

func HinkalSwap(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	deltaAmountsBase []*big.Int,
	externalActionID types.ExternalActionID,
	data string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	slippagePercentage float64,
	action types.AdminTransactionType,
) (string, error) {
	if slippagePercentage == 0 {
		slippagePercentage = constants.DefaultBridgingSlippage
	}
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	deltaAmounts := copyBigInts(deltaAmountsBase)
	erc20Addresses := tokenAddresses(erc20Tokens)

	var ethereumAddress string
	var feeStructure types.FeeStructure
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		ethereumAddress, err = hinkal.GetEthereumAddressByChain(groupCtx, chainID)
		return err
	})
	group.Go(func() error {
		if feeStructureOverride != nil {
			feeStructure = *feeStructureOverride
			return nil
		}
		resolved, err := pretransaction.GetFeeStructure(groupCtx, chainID, feeToken, erc20Addresses, externalActionID, nil, constants.HinkalSwapVariableRate(), nil)
		if err != nil {
			return err
		}
		feeStructure = resolved
		return nil
	})
	if err := group.Wait(); err != nil {
		return "", err
	}
	feeStructure = normalizeFeeStructure(feeStructure)
	if err := pretransaction.MergeWithFeeStructure(ctx, chainID, &erc20Addresses, &deltaAmounts, feeStructure); err != nil {
		return "", err
	}

	externalAddress, err := pretransaction.GetExternalSwapAddress(chainID, externalActionID)
	if err != nil {
		return "", err
	}

	adminData := pretransaction.ConstructAdminData(action, chainID, erc20Addresses, deltaAmounts, ethereumAddress, erc20Tokens, feeStructure.VariableRate)

	slippageValues := snarkjs.BuildSwapSlippageValues(deltaAmounts, slippagePercentage)

	if len(deltaAmounts) > 1 {
		deltaAmounts[1] = big.NewInt(0)
	}

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:                chainID,
		Erc20Addresses:         erc20Addresses,
		AmountChanges:          deltaAmounts,
		ExternalActionID:       externalActionID,
		ExternalAddress:        externalAddress,
		ExternalActionMetadata: []string{data},
		FeeStructure:           &feeStructure,
		OnChainCreation:        swapOnChainCreation(len(deltaAmounts)),
		SlippageValues:         slippageValues,
		Submit:                 NewRelayerSubmit(RelayerSubmit{AdminData: adminData}),
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
