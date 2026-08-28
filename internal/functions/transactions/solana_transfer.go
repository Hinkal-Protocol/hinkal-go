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

var errSolanaTransferOneMint = errors.New("Solana Transfer: Only one mint address is supported")

func resolveSolanaTransferFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	mintAddresses []string,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		var err error
		feeStructure, err = pretransaction.GetFeeStructure(
			ctx,
			chainID,
			feeToken,
			mintAddresses,
			types.ExternalActionTransact,
			nil,
			constants.HinkalPrivateSendVariableRate(),
			solanaTransactionParams,
		)
		if err != nil {
			return types.FeeStructure{}, err
		}
	}
	feeStructure = normalizeFeeStructure(feeStructure)
	if feeStructure.VariableRate.Sign() == 0 {
		feeStructure.VariableRate = constants.HinkalPrivateSendVariableRate()
	}
	return feeStructure, nil
}

func HinkalSolanaTransfer(
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
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}
	if len(erc20Tokens) != len(amountChangesBase) {
		return "", errTokenAmountLengthMismatch
	}
	if len(erc20Tokens) == 0 {
		return "", errTransferNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errSolanaTransferOneMint
	}
	if !pretransaction.IsValidPrivateAddress(recipientAddress) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}

	amountChanges := copyBigInts(amountChangesBase)
	mintAddresses := tokenAddresses(erc20Tokens)
	if err := ensureSolanaDeployData(chainID); err != nil {
		return "", err
	}
	if feeToken == "" {
		feeToken = mintAddresses[0]
	}
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         feeToken,
		NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
	}
	feeStructure, err := resolveSolanaTransferFeeStructure(ctx, chainID, feeToken, mintAddresses, feeStructureOverride, solanaParams)
	if err != nil {
		return "", err
	}
	recipientAmount := new(big.Int).Neg(amountChanges[0])
	totalFee := fees.CalculateTotalFee(recipientAmount, feeStructure)
	amountChanges[0] = new(big.Int).Sub(amountChanges[0], totalFee)

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}
	adminData := pretransaction.ConstructAdminData(types.AdminPrivateToPrivateSend, chainID, mintAddresses, amountChanges, ethereumAddress, nil)

	result, err := SolanaTransact(ctx, hinkal, HinkalSolanaTransactParams{
		ChainID:       chainID,
		MintAddresses: mintAddresses,
		AmountChanges: amountChanges,
		RelayAddress:  relay,
		Recipient:     constants.SolanaNativeAddress,
		Signer:        relay,
		FunctionName:  "transfer",
		Accounts: api.SolanaTransactAccounts{
			Recipient: relay,
			Mint:      nonNativeMintString(mintAddresses[0]),
		},
		RelayerFee:       totalFee,
		VariableRate:     big.NewInt(0),
		RecipientAddress: recipientAddress,
		RecipientAmounts: []*big.Int{recipientAmount},
		Submit:           SolanaTransactSubmit{Mode: SolanaSubmitModeRelayer, AdminData: adminData},
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}

func nonNativeMintString(mintAddress string) string {
	if mintAddress == constants.SolanaNativeAddress {
		return ""
	}
	return mintAddress
}
