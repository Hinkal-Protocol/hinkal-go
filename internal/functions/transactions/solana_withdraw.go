package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errSolanaWithdrawOneMint = errors.New("Solana Withdraw: Only one mint address is supported")
	errSolanaWithdrawNoToken = errors.New("Solana Withdraw: No Token Found")
)

func resolveSolanaWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	mintAddresses []string,
	token types.ERC20Token,
	amount *big.Int,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	var rawFeeStructure types.FeeStructure
	if feeStructureOverride != nil {
		rawFeeStructure = *feeStructureOverride
	} else {
		var err error
		rawFeeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, mintAddresses, types.ExternalActionTransact, nil, nil, solanaTransactionParams)
		if err != nil {
			return types.FeeStructure{}, err
		}
	}
	return fees.CalculateModifiedFeeStructure(ctx, chainID, token, new(big.Int).Neg(amount), normalizeFeeStructure(rawFeeStructure)), nil
}

func HinkalSolanaWithdraw(
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
		return "", errSolanaWithdrawNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errSolanaWithdrawOneMint
	}

	amountChanges := copyBigInts(amountChangesBase)
	mintAddresses := tokenAddresses(erc20Tokens)
	token := erc20Tokens[0]
	if err := ensureSolanaDeployData(chainID); err != nil {
		return "", err
	}
	if feeToken == "" {
		feeToken = mintAddresses[0]
	}
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         mintAddresses[0],
		Recipient:      recipientAddress,
		NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
	}
	feeStructure, err := resolveSolanaWithdrawFeeStructure(ctx, chainID, feeToken, mintAddresses, token, amountChanges[0], feeStructureOverride, solanaParams)
	if err != nil {
		return "", err
	}
	amountChanges[0] = new(big.Int).Sub(amountChanges[0], feeStructure.FlatFee)

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	accounts := api.SolanaTransactAccounts{Recipient: recipientAddress}
	if !strings.EqualFold(mintAddresses[0], constants.SolanaNativeAddress) {
		accounts.Mint = mintAddresses[0]
	}

	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}
	adminData := pretransaction.ConstructAdminData(types.AdminUnshield, chainID, mintAddresses, amountChanges, ethereumAddress, nil)

	result, err := SolanaTransact(ctx, hinkal, HinkalSolanaTransactParams{
		ChainID:       chainID,
		MintAddresses: mintAddresses,
		AmountChanges: amountChanges,
		RelayAddress:  relay,
		Recipient:     recipientAddress,
		Signer:        relay,
		FunctionName:  "transact",
		Accounts:      accounts,
		RelayerFee:    feeStructure.FlatFee,
		VariableRate:  feeStructure.VariableRate,
		Submit:        SolanaTransactSubmit{Mode: SolanaSubmitModeRelayer, AdminData: adminData},
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
