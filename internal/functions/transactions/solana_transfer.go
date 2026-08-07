package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errSolanaTransferOneMint       = errors.New("Solana Transfer: Only one mint address is supported")
	errSolanaTransferMissingOutput = errors.New("transactions: Solana transfer missing output UTXO")
)

func getSolanaTransferInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	recipientAmount *big.Int,
) ([]*utxo.Utxo, []*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, false, false)
	if err != nil {
		return nil, nil, err
	}
	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[0], amountChanges[0], timeStamp, true, recipientAddress, recipientAmount)
	if err != nil {
		return nil, nil, err
	}
	if len(outputUtxos) < 2 {
		return nil, nil, errSolanaTransferMissingOutput
	}
	return inputUtxosArray[0], outputUtxos, nil
}

func solanaTransferEncryptedOutputs(outputUtxos []*utxo.Utxo) ([][]byte, [][]int, error) {
	encryptedOutputs, err := snarkjs.CalcEncryptedOutputs([][]*utxo.Utxo{outputUtxos})
	if err != nil {
		return nil, nil, err
	}
	if len(encryptedOutputs) == 0 || len(encryptedOutputs[0]) != len(outputUtxos) {
		return nil, nil, errSolanaTransferMissingOutput
	}
	bytesArr := make([][]byte, len(encryptedOutputs[0]))
	intsArr := make([][]int, len(encryptedOutputs[0]))
	for i, encryptedOutput := range encryptedOutputs[0] {
		row := common.FromHex(encryptedOutput)
		bytesArr[i] = row
		ints := make([]int, len(row))
		for j, b := range row {
			ints[j] = int(b)
		}
		intsArr[i] = ints
	}
	return bytesArr, intsArr, nil
}

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
	inputUtxos, outputUtxos, err := getSolanaTransferInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, recipientAddress, recipientAmount)
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
	encryptedOutputBytes, encryptedOutputInts, err := solanaTransferEncryptedOutputs(outputUtxos)
	if err != nil {
		return "", err
	}
	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	if err := pretransaction.EnsureAmountChanges(inputUtxosArray, [][]*utxo.Utxo{{outputUtxos[0]}}, amountChanges); err != nil {
		return "", err
	}

	dimensions := types.DimDataType{
		TokenNumber:     len(mintAddresses),
		NullifierAmount: len(inputUtxos),
		OutputAmount:    len(outputUtxos),
	}
	proof, err := snarkjs.ConstructSolanaZkProof(ctx, snarkjs.ConstructSolanaZkProofParams{
		GenerateProofRemotely: hinkal.GenerateProofRemotely(),
		MerkleTree:            hinkal.MerkleTree(chainID),
		UserKeys:              hinkal.GetUserKeys(),
		MintAddresses:         mintAddresses,
		InputUtxos:            inputUtxosArray,
		OutputUtxos:           [][]*utxo.Utxo{outputUtxos},
		ExtraRandomization:    extraRandomization,
		RelayerFee:            totalFee,
		VariableRate:          big.NewInt(0),
		RecipientAddress:      constants.SolanaNativeAddress,
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
	})
	if err != nil {
		return "", err
	}

	ethereumAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}
	adminData := pretransaction.ConstructAdminData(types.AdminPrivateToPrivateSend, chainID, mintAddresses, amountChanges, ethereumAddress, nil)

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:      chainID,
		RelayAddress: relay,
		FunctionName: "transfer",
		Args: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       totalFee.String(),
			Dimensions:       dimensions,
		},
		Accounts: api.SolanaTransactAccounts{
			Recipient: relay,
			Mint:      nonNativeMintString(mintAddresses[0]),
		},
		CommitmentValidationData: proof.CommitmentValidationData,
		AdminData:                adminData,
	})
}

func nonNativeMintString(mintAddress string) string {
	if mintAddress == constants.SolanaNativeAddress {
		return ""
	}
	return mintAddress
}
