package transactions

import (
	"context"
	"errors"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func solanaSwapSlippagePercent(hinkal ihinkal.HinkalInternal) float64 {
	raw, ok := hinkal.CacheDevice().Get(constants.StorageKeys.SlippagePercentage)
	if !ok {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
}

var errSolanaSwapTwoTokens = errors.New("transactions: Solana swap requires exactly two tokens")

func getSolanaSwapInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) ([][]*utxo.Utxo, [][]*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, false, false)
	if err != nil {
		return nil, nil, err
	}
	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxosArray := make([][]*utxo.Utxo, len(mintAddresses))
	for i := range mintAddresses {
		outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[i], amountChanges[i], timeStamp, true, "", nil)
		if err != nil {
			return nil, nil, err
		}
		outputUtxosArray[i] = []*utxo.Utxo{outputUtxos[0]}
	}
	return inputUtxosArray, outputUtxosArray, nil
}

func solanaSwapEncryptedOutputs(outputUtxosArray [][]*utxo.Utxo) ([][]byte, [][]int, error) {
	encryptedOutputs, err := snarkjs.CalcEncryptedOutputs(outputUtxosArray)
	if err != nil {
		return nil, nil, err
	}
	bytesArr := make([][]byte, len(encryptedOutputs))
	intsArr := make([][]int, len(encryptedOutputs))
	for i, tokenOutputs := range encryptedOutputs {
		if len(tokenOutputs) == 0 {
			return nil, nil, errSolanaWithdrawMissingOutput
		}
		row := common.FromHex(tokenOutputs[0])
		bytesArr[i] = row
		ints := make([]int, len(row))
		for j, b := range row {
			ints[j] = int(b)
		}
		intsArr[i] = ints
	}
	return bytesArr, intsArr, nil
}

func HinkalSolanaSwap(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	swapperAccountSalt *big.Int,
	instructionLists []api.OKXSwapResponseInstruction,
	addressLookupTableAccount []string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
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
	if feeToken == "" {
		feeToken = mintAddresses[1]
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		solanaParams := &api.SolanaGasEstimateParams{
			MintTo:         feeToken,
			MintFrom:       mintAddresses[0],
			NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
		}
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, mintAddresses, types.ExternalActionOkx, nil, nil, solanaParams)
		if err != nil {
			return "", err
		}
	}
	feeStructure = normalizeFeeStructure(feeStructure)

	initialReceiveAmount := amountChanges[1]
	outputToken := erc20Tokens[1]
	outputAmount, err := strconv.ParseFloat(web3.GetAmountInToken(outputToken, initialReceiveAmount), 64)
	if err != nil {
		return "", err
	}
	slippageAmount := outputAmount * solanaSwapSlippagePercent(hinkal) / 100
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

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	inputUtxosArray, outputUtxosArray, err := getSolanaSwapInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges)
	if err != nil {
		return "", err
	}

	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
	if err != nil {
		return "", err
	}

	encryptedOutputBytes, encryptedOutputInts, err := solanaSwapEncryptedOutputs(outputUtxosArray)
	if err != nil {
		return "", err
	}

	swapperAccount, err := web3.GetSwapperAccountPublicKeyFromSalt(hinkalProgramAddress, originalDeployer, swapperAccountSalt)
	if err != nil {
		return "", err
	}
	hinkalInstructions, remainingAccounts, err := pretransaction.ConvertOKXToHinkalInstructions(instructionLists, swapperAccount)
	if err != nil {
		return "", err
	}

	if err := pretransaction.EnsureAmountChanges(inputUtxosArray, outputUtxosArray, amountChanges); err != nil {
		return "", err
	}

	dimensions := types.DimDataType{
		TokenNumber:     2,
		NullifierAmount: len(inputUtxosArray[0]),
		OutputAmount:    1,
	}
	proof, err := snarkjs.ConstructSolanaZkProof(ctx, snarkjs.ConstructSolanaZkProofParams{
		GenerateProofRemotely: hinkal.GenerateProofRemotely(),
		MerkleTree:            hinkal.MerkleTree(chainID),
		UserKeys:              hinkal.GetUserKeys(),
		MintAddresses:         mintAddresses,
		InputUtxos:            inputUtxosArray,
		OutputUtxos:           outputUtxosArray,
		ExtraRandomization:    extraRandomization,
		RelayerFee:            feeStructure.FlatFee,
		VariableRate:          feeStructure.VariableRate,
		RecipientAddress:      recipient.String(),
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
		Instructions:          hinkalInstructions,
		RemainingAccounts:     remainingAccounts,
		SwapperAccountSalt:    swapperAccountSalt,
	})
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

	variableRate := feeStructure.VariableRate.String()
	args := api.SolanaSwapArgs{
		SolanaArgs: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       feeStructure.FlatFee.String(),
			VariableRate:     &variableRate,
			Dimensions:       dimensions,
		},
		HinkalInstructions: hinkalInstructionsToAPI(hinkalInstructions),
	}

	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}
	adminData := pretransaction.ConstructAdminData(types.AdminPrivateSwap, chainID, mintAddresses, amountChanges, ethereumAddress, erc20Tokens)

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:                  chainID,
		RelayAddress:             relay,
		FunctionName:             "swap",
		Args:                     args,
		Accounts:                 accounts,
		CommitmentValidationData: proof.CommitmentValidationData,
		AdminData:                adminData,
	})
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
