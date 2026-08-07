package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errTransferNoToken = errors.New("transactions: transfer action: no token found")

func resolveTransferFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	erc20Addresses []string,
	sentToken types.ERC20Token,
	recipientAmount *big.Int,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	var rawFeeStructure types.FeeStructure
	if feeStructureOverride != nil {
		rawFeeStructure = *feeStructureOverride
	} else {
		var err error
		rawFeeStructure, err = pretransaction.GetFeeStructure(
			ctx,
			chainID,
			feeToken,
			erc20Addresses,
			types.ExternalActionTransact,
			nil,
			constants.HinkalPrivateSendVariableRate(),
			solanaTransactionParams,
		)
		if err != nil {
			return types.FeeStructure{}, err
		}
	}
	rawFeeStructure = normalizeFeeStructure(rawFeeStructure)
	if rawFeeStructure.VariableRate.Sign() == 0 {
		rawFeeStructure.VariableRate = constants.HinkalPrivateSendVariableRate()
	}
	return fees.CalculateModifiedFeeStructure(ctx, chainID, sentToken, recipientAmount, rawFeeStructure), nil
}

func transferRecipientAmounts(erc20Addresses []string, recipientAmount *big.Int) []*big.Int {
	amounts := make([]*big.Int, len(erc20Addresses))
	for i := range erc20Addresses {
		if i == 0 {
			amounts[i] = recipientAmount
			continue
		}
		amounts[i] = big.NewInt(0)
	}
	return amounts
}

func HinkalTransfer(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	recipientAddress string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if len(erc20Tokens) != len(amountChangesBase) {
		return "", errTokenAmountLengthMismatch
	}
	if len(erc20Tokens) == 0 {
		return "", errTransferNoToken
	}
	if !pretransaction.IsValidPrivateAddress(recipientAddress) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}

	amountChanges := pretransaction.ModifyVolatileTokenAmountChanges(ctx, chainID, erc20Tokens, copyBigInts(amountChangesBase), "")
	erc20Addresses := tokenAddresses(erc20Tokens)
	sentToken := erc20Tokens[0]
	recipientAmount := new(big.Int).Neg(amountChanges[0])

	feeStructure, err := resolveTransferFeeStructure(ctx, chainID, feeToken, erc20Addresses, sentToken, recipientAmount, feeStructureOverride, nil)
	if err != nil {
		return "", err
	}
	if err := pretransaction.MergeWithFeeStructure(ctx, chainID, &erc20Addresses, &amountChanges, feeStructure); err != nil {
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
	adminData := pretransaction.ConstructAdminData(types.AdminPrivateToPrivateSend, chainID, erc20Addresses, amountChanges, ethereumAddress, nil)

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:          chainID,
		Erc20Addresses:   erc20Addresses,
		AmountChanges:    amountChanges,
		ExternalActionID: types.ExternalActionZero,
		ExternalAddress:  relay,
		FeeStructure:     &feeStructure,
		Relay:            relay,
		RecipientAddress: recipientAddress,
		RecipientAmounts: transferRecipientAmounts(erc20Addresses, recipientAmount),
		Submit:           NewRelayerSubmit(RelayerSubmit{AdminData: adminData}),
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
