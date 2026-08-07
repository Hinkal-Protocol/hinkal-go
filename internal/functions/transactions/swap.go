package transactions

import (
	"context"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
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
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	deltaAmounts := copyBigInts(deltaAmountsBase)
	erc20Addresses := tokenAddresses(erc20Tokens)

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, erc20Addresses, externalActionID, nil, constants.HinkalSwapVariableRate(), nil)
		if err != nil {
			return "", err
		}
	}
	feeStructure = normalizeFeeStructure(feeStructure)
	if err := pretransaction.MergeWithFeeStructure(ctx, chainID, &erc20Addresses, &deltaAmounts, feeStructure); err != nil {
		return "", err
	}

	externalAddress, err := pretransaction.GetExternalSwapAddress(chainID, externalActionID)
	if err != nil {
		return "", err
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return "", err
	}
	adminData := pretransaction.ConstructAdminData(types.AdminPrivateSwap, chainID, erc20Addresses, deltaAmounts, ethereumAddress, erc20Tokens)

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:                chainID,
		Erc20Addresses:         erc20Addresses,
		AmountChanges:          deltaAmounts,
		ExternalActionID:       externalActionID,
		ExternalAddress:        externalAddress,
		ExternalActionMetadata: []string{data},
		FeeStructure:           &feeStructure,
		Relay:                  relay,
		OnChainCreation:        swapOnChainCreation(len(deltaAmounts)),
		Submit:                 NewRelayerSubmit(RelayerSubmit{AdminData: adminData}),
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
