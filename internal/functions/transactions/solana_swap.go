package transactions

import (
	"context"
	"errors"
	"math/big"
	"strconv"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errSolanaSwapTwoTokens = errors.New("transactions: Solana swap requires exactly two tokens")

func HinkalSolanaSwap(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	swapperAccountSalt *big.Int,
	instructionLists []api.OKXSwapResponseInstruction,
	addressLookupTableAccount []string,
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
	if len(erc20Tokens) != 2 || len(amountChangesBase) != 2 {
		return "", errSolanaSwapTwoTokens
	}

	recipient, err := solana.PublicKeyFromBase58(constants.SolanaNativeAddress)
	if err != nil {
		return "", err
	}
	hinkalAddressStr, err := constants.HinkalAddress(chainID)
	if err != nil {
		return "", err
	}
	hinkalProgramAddress, err := solana.PublicKeyFromBase58(hinkalAddressStr)
	if err != nil {
		return "", err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return "", err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return "", err
	}

	amountChanges := copyBigInts(amountChangesBase)
	mintAddresses := tokenAddresses(erc20Tokens)

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		solanaParams := &api.SolanaGasEstimateParams{
			MintTo:         mintAddresses[1],
			MintFrom:       mintAddresses[0],
			NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
		}
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, mintAddresses[1], mintAddresses, types.ExternalActionOkx, nil, nil, solanaParams)
		if err != nil {
			return "", err
		}
	}
	if err := fees.AssertSolanaFeeTokenMatchesMint(feeStructure, mintAddresses[1]); err != nil {
		return "", err
	}
	feeStructure = normalizeFeeStructure(feeStructure)

	initialReceiveAmount := amountChanges[1]
	outputToken := erc20Tokens[1]
	outputAmount, err := strconv.ParseFloat(web3.GetAmountInToken(outputToken, initialReceiveAmount), 64)
	if err != nil {
		return "", err
	}
	slippageAmount := outputAmount * slippagePercentage / 100
	slippageWei, err := web3.GetAmountInWei(outputToken, strconv.FormatFloat(slippageAmount, 'f', outputToken.Decimals, 64))
	if err != nil {
		return "", err
	}
	amountChanges[1] = new(big.Int).Sub(amountChanges[1], slippageWei)

	variableFee := new(big.Int).Quo(new(big.Int).Mul(initialReceiveAmount, feeStructure.VariableRate), big.NewInt(10000))
	totalFee := new(big.Int).Add(variableFee, feeStructure.FlatFee)
	amountChanges[1] = new(big.Int).Sub(amountChanges[1], totalFee)
	if amountChanges[1].Sign() < 0 {
		return "", errorhandling.ErrLowOutputAmount
	}

	swapperAccount, err := web3.GetSwapperAccountPublicKeyFromSalt(hinkalProgramAddress, originalDeployer, swapperAccountSalt)
	if err != nil {
		return "", err
	}
	hinkalInstructions, remainingAccounts, err := pretransaction.ConvertOKXToHinkalInstructions(instructionLists, swapperAccount)
	if err != nil {
		return "", err
	}

	storageAccount, err := web3.GetStorageAccountPublicKey(hinkalProgramAddress, originalDeployer)
	if err != nil {
		return "", err
	}
	storageVault, err := web3.GetStorageVaultPublicKey(hinkalProgramAddress, originalDeployer)
	if err != nil {
		return "", err
	}

	accounts := api.SolanaSwapAccounts{
		Recipient:                 recipient.String(),
		StorageAccount:            storageAccount.String(),
		StorageVault:              storageVault.String(),
		SwapperAccount:            swapperAccount.String(),
		MintFrom:                  nonNativeMint(mintAddresses[0]),
		MintTo:                    nonNativeMint(mintAddresses[1]),
		RemainingAccounts:         remainingAccountsToOKX(remainingAccounts),
		AddressLookupTableAccount: addressLookupTableAccount,
	}

	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}
	adminData := pretransaction.ConstructAdminData(action, chainID, mintAddresses, amountChanges, ethereumAddress, erc20Tokens, feeStructure.VariableRate)

	result, err := SolanaTransact(ctx, hinkal, HinkalSolanaTransactParams{
		ChainID:            chainID,
		MintAddresses:      mintAddresses,
		AmountChanges:      amountChanges,
		Recipient:          recipient.String(),
		FunctionName:       "swap",
		Accounts:           accounts,
		OnChainCreation:    []bool{false, true},
		RelayerFee:         feeStructure.FlatFee,
		VariableRate:       feeStructure.VariableRate,
		SwapperAccountSalt: swapperAccountSalt,
		HinkalInstructions: hinkalInstructions,
		RemainingAccounts:  remainingAccounts,
		Submit:             SolanaTransactSubmit{Mode: SolanaSubmitModeRelayer, AdminData: adminData},
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}

func nonNativeMint(mintAddress string) *string {
	if mintAddress == constants.SolanaNativeAddress {
		return nil
	}
	mint := mintAddress
	return &mint
}

func remainingAccountsToOKX(remainingAccounts []solana.AccountMeta) []api.OKXAccount {
	out := make([]api.OKXAccount, len(remainingAccounts))
	for i, acc := range remainingAccounts {
		out[i] = api.OKXAccount{
			IsSigner:   acc.IsSigner,
			IsWritable: acc.IsWritable,
			Pubkey:     acc.PublicKey.String(),
		}
	}
	return out
}

func hinkalInstructionsToAPI(instructions []solanautils.HinkalInstruction) []api.SolanaHinkalInstruction {
	out := make([]api.SolanaHinkalInstruction, len(instructions))
	for i, inst := range instructions {
		accountIndexes := make([]int, len(inst.AccountIndexes))
		for j, b := range inst.AccountIndexes {
			accountIndexes[j] = int(b)
		}
		data := make([]int, len(inst.Data))
		for j, b := range inst.Data {
			data[j] = int(b)
		}
		out[i] = api.SolanaHinkalInstruction{
			AccountIndexes: accountIndexes,
			Data:           data,
			ProgramIndex:   inst.ProgramIndex,
		}
	}
	return out
}
