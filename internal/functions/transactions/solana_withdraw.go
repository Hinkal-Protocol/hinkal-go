package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errSolanaWithdrawOneMint       = errors.New("Solana Withdraw: Only one mint address is supported")
	errSolanaWithdrawNoToken       = errors.New("Solana Withdraw: No Token Found")
	errSolanaWithdrawMissingOutput = errors.New("transactions: Solana withdraw missing output UTXO")
)

func getSolanaWithdrawInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) ([]*utxo.Utxo, []*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, false, false)
	if err != nil {
		return nil, nil, err
	}
	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[0], amountChanges[0], timeStamp, true, "", nil)
	if err != nil {
		return nil, nil, err
	}
	return inputUtxosArray[0], outputUtxos, nil
}

func solanaEncryptedOutputBytes(outputUtxos []*utxo.Utxo) ([][]byte, [][]int, error) {
	encryptedOutputs, err := snarkjs.CalcEncryptedOutputs([][]*utxo.Utxo{outputUtxos})
	if err != nil {
		return nil, nil, err
	}
	if len(encryptedOutputs) == 0 || len(encryptedOutputs[0]) == 0 {
		return nil, nil, errSolanaWithdrawMissingOutput
	}
	encryptedOutputBytes := common.FromHex(encryptedOutputs[0][0])
	bytes := [][]byte{encryptedOutputBytes}
	ints := make([][]int, len(bytes))
	for i, row := range bytes {
		ints[i] = make([]int, len(row))
		for j, b := range row {
			ints[i][j] = int(b)
		}
	}
	return bytes, ints, nil
}

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
	inputUtxos, outputUtxos, err := getSolanaWithdrawInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges)
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
	encryptedOutputBytes, encryptedOutputInts, err := solanaEncryptedOutputBytes(outputUtxos)
	if err != nil {
		return "", err
	}
	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	outputUtxosArray := [][]*utxo.Utxo{outputUtxos}
	if err := pretransaction.EnsureAmountChanges(inputUtxosArray, outputUtxosArray, amountChanges); err != nil {
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
		OutputUtxos:           outputUtxosArray,
		ExtraRandomization:    extraRandomization,
		RelayerFee:            feeStructure.FlatFee,
		VariableRate:          feeStructure.VariableRate,
		RecipientAddress:      recipientAddress,
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
	})
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

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:      chainID,
		RelayAddress: relay,
		FunctionName: "transact",
		Args: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       feeStructure.FlatFee.String(),
			Dimensions:       dimensions,
		},
		Accounts:                 accounts,
		CommitmentValidationData: proof.CommitmentValidationData,
		AdminData:                adminData,
	})
}
