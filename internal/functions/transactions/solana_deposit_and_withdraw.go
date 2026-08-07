package transactions

import (
	"context"
	"math/big"
	"strings"

	solana "github.com/gagliardetto/solana-go"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/onchainutxos"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func resolveSolanaDepositAndWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	mintAddress string,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return normalizeFeeStructure(*feeStructureOverride), nil
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		mintAddress,
		[]string{mintAddress},
		types.ExternalActionTransact,
		nil,
		constants.HinkalPrivateSendVariableRate(),
		solanaTransactionParams,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return normalizeFeeStructure(feeStructure), nil
}

func solanaDepositAndWithdrawUtxoAmounts(recipientAmounts []*big.Int, feeStructure types.FeeStructure) []*big.Int {
	amounts := make([]*big.Int, len(recipientAmounts))
	for i, amount := range recipientAmounts {
		amounts[i] = new(big.Int).Add(amount, fees.CalculateTotalFee(amount, feeStructure))
	}
	return amounts
}

func hinkalSolanaMultiPaymentDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
) ([]recipientUtxo, string, string, error) {
	amounts := solanaDepositAndWithdrawUtxoAmounts(recipientAmounts, feeStructure)
	structures, err := getProoflessStealthAddressStructures(hinkal, len(amounts), nil)
	if err != nil {
		return nil, "", "", err
	}
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return nil, "", "", err
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return nil, "", "", err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return nil, "", "", err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return nil, "", "", err
	}
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return nil, "", "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return nil, "", "", err
	}

	instruction, err := buildMultiPaymentDepositInstruction(programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, true)
	if err != nil {
		return nil, "", "", err
	}

	statusResp, err := api.UpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeDeposit,
	})
	if err != nil {
		return nil, "", "", err
	}

	signature, err := signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, []solana.Instruction{instruction})
	if err != nil {
		return nil, "", "", err
	}
	if _, err := hinkal.WaitForTransaction(ctx, chainID, signature, 1); err != nil {
		return nil, "", "", err
	}
	tx, err := solanautils.FetchTransactionWithRetry(ctx, chainID, signature)
	if err != nil {
		return nil, "", "", err
	}

	api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusResp.ID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseAfterDeposit,
		DepositTxHash:         signature,
	})

	formattedMint, err := solanautils.FormatMintAddress(token.Erc20TokenAddress)
	if err != nil {
		return nil, "", "", err
	}
	depositedUtxos, err := onchainutxos.DecodeSolanaFromTransaction(tx, hinkal.GetUserKeys(), formattedMint.CompressedAddress)
	if err != nil {
		return nil, "", "", err
	}
	if len(depositedUtxos) == 0 {
		return nil, "", "", errNoUtxosInDepositTransaction
	}
	userDepositedUtxos, err := matchRecipientUtxos(recipientAddresses, amounts, depositedUtxos)
	if err != nil {
		return nil, "", "", err
	}
	return userDepositedUtxos, statusResp.ID, signature, nil
}

func hinkalSolanaWithdrawBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	hashedEthereumAddress string,
	recipientAmounts []*big.Int,
	statusID string,
	txCompletionTime *int,
) (string, error) {
	if len(userDepositedUtxos) == 0 {
		return "", errUserDepositedUtxosEmpty
	}
	mintAddress := token.Erc20TokenAddress
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	withdrawTimeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	transactions := make([]api.SolanaTransactionBody, 0, len(userDepositedUtxos))

	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}

	generateProofRemotely := hinkal.GenerateProofRemotely()
	batchSize := proofBatchSize(generateProofRemotely)
	for start := 0; start < len(userDepositedUtxos); start += batchSize {
		end := start + batchSize
		if end > len(userDepositedUtxos) {
			end = len(userDepositedUtxos)
		}
		batchItems := userDepositedUtxos[start:end]
		batchTransactions := make([]api.SolanaTransactionBody, len(batchItems))
		group, groupCtx := errgroup.WithContext(ctx)

		for batchIndex, item := range batchItems {
			batchIndex, absoluteIndex, item := batchIndex, start+batchIndex, item
			group.Go(func() error {
				zeroUtxo, err := buildSolanaZeroUtxo(hinkal, mintAddress, withdrawTimeStamp)
				if err != nil {
					return err
				}
				withdrawInputUtxos := []*utxo.Utxo{item.utxo, zeroUtxo}
				withdrawOutputUtxos := []*utxo.Utxo{zeroUtxo}
				randSeed, err := utils.RandomBigInt(31)
				if err != nil {
					return err
				}
				extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
				if err != nil {
					return err
				}
				encryptedOutputBytes, encryptedOutputInts, err := solanaEncryptedOutputBytes(withdrawOutputUtxos)
				if err != nil {
					return err
				}
				inputUtxosArray := [][]*utxo.Utxo{withdrawInputUtxos}
				outputUtxosArray := [][]*utxo.Utxo{withdrawOutputUtxos}
				dimensions := types.DimDataType{
					TokenNumber:     1,
					NullifierAmount: len(withdrawInputUtxos),
					OutputAmount:    len(withdrawOutputUtxos),
				}
				finalFeeStructure := fees.CalculateModifiedFeeStructure(groupCtx, chainID, token, recipientAmounts[absoluteIndex], feeStructure)
				proof, err := snarkjs.ConstructSolanaZkProof(groupCtx, snarkjs.ConstructSolanaZkProofParams{
					GenerateProofRemotely: generateProofRemotely,
					MerkleTree:            hinkal.MerkleTree(chainID),
					UserKeys:              hinkal.GetUserKeys(),
					MintAddresses:         []string{mintAddress},
					InputUtxos:            inputUtxosArray,
					OutputUtxos:           outputUtxosArray,
					ExtraRandomization:    extraRandomization,
					RelayerFee:            finalFeeStructure.FlatFee,
					VariableRate:          finalFeeStructure.VariableRate,
					RecipientAddress:      item.recipientAddress,
					SignerAddress:         relay,
					Dimensions:            dimensions,
					EncryptedOutputs:      encryptedOutputBytes,
					ChainID:               chainID,
				})
				if err != nil {
					return err
				}

				accounts := api.SolanaTransactAccounts{Recipient: item.recipientAddress}
				if !strings.EqualFold(mintAddress, constants.SolanaNativeAddress) {
					accounts.Mint = mintAddress
				}
				adminData := pretransaction.ConstructAdminData(
					types.AdminPublicToPublicSend,
					chainID,
					[]string{mintAddress},
					[]*big.Int{recipientAmounts[absoluteIndex]},
					ethereumAddress,
					nil,
				)
				batchTransactions[batchIndex] = api.SolanaTransactionBody{
					ChainID:         chainID,
					RelayAddress:    relay,
					FunctionName:    "transact",
					RecipientAmount: recipientAmounts[absoluteIndex].String(),
					Args: api.SolanaArgs{
						ProofAArr:        proof.ProofAArr,
						ProofBArr:        proof.ProofBArr,
						ProofCArr:        proof.ProofCArr,
						PublicInputsArr:  proof.PublicInputsArr,
						EncryptedOutputs: encryptedOutputInts,
						RelayerFee:       finalFeeStructure.FlatFee.String(),
						Dimensions:       dimensions,
					},
					Accounts:                 accounts,
					CommitmentValidationData: proof.CommitmentValidationData,
					AdminData:                adminData,
				}
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return "", err
		}
		transactions = append(transactions, batchTransactions...)
	}

	api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeScheduleWithdraw,
	})
	scheduleID, err := web3.SolanaTransactCallRelayerBatch(ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "")
	if err != nil {
		return "", err
	}
	api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseAfterScheduleWithdraw,
		ScheduleID:            scheduleID,
	})
	return scheduleID, nil
}

func HinkalSolanaDepositAndWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
) (types.DepositAndSendExtendedResult, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if !constants.IsSolanaLike(chainID) {
		return types.DepositAndSendExtendedResult{}, errNotSolanaChain
	}
	if err := validateDepositAndWithdrawArgs(erc20Tokens, recipientAmounts, recipientAddresses); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	token := erc20Tokens[0]
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(rawEthereumAddress)
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         token.Erc20TokenAddress,
		NullifierCount: 1,
	}
	feeStructure, err := resolveSolanaDepositAndWithdrawFeeStructure(ctx, chainID, token.Erc20TokenAddress, feeStructureOverride, solanaParams)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	userDepositedUtxos, statusID, depositTxHash, err := hinkalSolanaMultiPaymentDeposit(
		ctx,
		hinkal,
		chainID,
		token,
		recipientAmounts,
		recipientAddresses,
		feeStructure,
		hashedEthereumAddress,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if err := waitForDepositedUtxosInMerkleTree(ctx, hinkal, chainID, userDepositedUtxos); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	scheduleID, err := hinkalSolanaWithdrawBatch(
		ctx,
		hinkal,
		chainID,
		token,
		userDepositedUtxos,
		feeStructure,
		rawEthereumAddress,
		hashedEthereumAddress,
		recipientAmounts,
		statusID,
		txCompletionTime,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}
