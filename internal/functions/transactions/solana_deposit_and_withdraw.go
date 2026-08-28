package transactions

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	solana "github.com/gagliardetto/solana-go"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
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

func solanaWithdrawBatchPrepare(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	recipientAmounts []*big.Int,
	pendingLeaves []string,
) ([]api.SolanaTransactionBody, error) {
	if len(userDepositedUtxos) == 0 {
		return nil, errUserDepositedUtxosEmpty
	}
	mintAddress := token.Erc20TokenAddress
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return nil, err
	}
	withdrawTimeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	transactions := make([]api.SolanaTransactionBody, 0, len(userDepositedUtxos))

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
				finalFeeStructure := fees.CalculateModifiedFeeStructure(groupCtx, chainID, token, recipientAmounts[absoluteIndex], feeStructure)

				var speculative *types.SpeculativeTreeParams
				if pendingLeaves != nil {
					speculativeInputs, err := pretransaction.ToSpeculativeUtxos([][]*utxo.Utxo{{item.utxo}})
					if err != nil {
						return err
					}
					speculative = &types.SpeculativeTreeParams{PendingLeaves: pendingLeaves, InputUtxos: speculativeInputs}
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

				result, err := SolanaTransact(groupCtx, hinkal, HinkalSolanaTransactParams{
					ChainID:         chainID,
					MintAddresses:   []string{mintAddress},
					AmountChanges:   []*big.Int{new(big.Int).Neg(item.utxo.Amount)},
					RelayAddress:    relay,
					Recipient:       item.recipientAddress,
					Signer:          relay,
					FunctionName:    "transact",
					Accounts:        accounts,
					RelayerFee:      finalFeeStructure.FlatFee,
					VariableRate:    finalFeeStructure.VariableRate,
					UseBlockedUtxos: true,
					InputUtxos:      [][]*utxo.Utxo{withdrawInputUtxos},
					Speculative:     speculative,
					Submit:          SolanaTransactSubmit{Mode: SolanaSubmitModeProofOnly},
				})
				if err != nil {
					return err
				}

				batchTransactions[batchIndex] = api.SolanaTransactionBody{
					ChainID:                  chainID,
					RelayAddress:             relay,
					FunctionName:             "transact",
					RecipientAmount:          recipientAmounts[absoluteIndex].String(),
					Args:                     result.Proof.Args,
					Accounts:                 accounts,
					CommitmentValidationData: result.Proof.CommitmentValidationData,
					AdminData:                adminData,
				}
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return nil, err
		}
		transactions = append(transactions, batchTransactions...)
	}

	return transactions, nil
}

func HinkalSolanaWithdrawBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	hashedEthereumAddress string,
	recipientAmounts []*big.Int,
	txCompletionTime *int,
	pendingLeaves []string,
	dependsOnTxHash string,
) (string, error) {
	transactions, err := solanaWithdrawBatchPrepare(
		ctx, hinkal, chainID, token, userDepositedUtxos, feeStructure, ethereumAddress, recipientAmounts, pendingLeaves,
	)
	if err != nil {
		return "", err
	}
	return web3.SolanaTransactCallRelayerBatch(
		ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "", dependsOnTxHash,
	)
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
	mintAddress := token.Erc20TokenAddress
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(rawEthereumAddress)
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         mintAddress,
		NullifierCount: 1,
	}
	feeStructure, err := resolveSolanaDepositAndWithdrawFeeStructure(ctx, chainID, mintAddress, feeStructureOverride, solanaParams)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	amounts := solanaDepositAndWithdrawUtxoAmounts(recipientAmounts, feeStructure)
	structures, err := getProoflessStealthAddressStructures(hinkal, len(amounts), nil)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	timeStampSeconds := utils.GetCurrentTimeInSeconds()
	timeStamp := strconv.FormatInt(timeStampSeconds, 10)

	formattedMint, err := solanautils.FormatMintAddress(mintAddress)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	nullifyingKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	depositedUtxos := make([]recipientUtxo, len(structures))
	pendingLeaves := make([]string, len(structures))
	for i, structure := range structures {
		note, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            amounts[i],
			TimeStamp:         timeStamp,
			NullifyingKey:     nullifyingKey,
			MintAddress:       mintAddress,
			Erc20TokenAddress: formattedMint.CompressedAddress,
			StealthAddress:    utils.ToBeHex(structure.StealthAddress),
			H0:                &types.JubPoint{structure.H0x, structure.H0y},
		})
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
		commitment, err := note.GetCommitment()
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
		commitmentBig, err := utils.ParseBigInt(commitment)
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
		depositedUtxos[i] = recipientUtxo{recipientAddress: recipientAddresses[i], utxo: note}
		pendingLeaves[i] = commitmentBig.String()
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	ownEncryptionKeys, err := getOwnRecipientEncryptionKeys(hinkal, len(structures))
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	encryptedBlobs, err := buildSolanaDepositEncryptedBlobs(structures, ownEncryptionKeys)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	depositInstruction, err := buildMultiPaymentDepositInstruction(
		ctx, connection, programID, signer, originalDeployer, mintAddress, amounts, structures, encryptedBlobs, true, &timeStampSeconds,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	var depositTxHash string
	var batchTransactions []api.SolanaTransactionBody
	// Plain errgroup, not WithContext: a failing proof must not cancel a deposit already in flight.
	var group errgroup.Group
	group.Go(func() error {
		var err error
		depositTxHash, err = sendSolanaInstructions(ctx, hinkal, programID, connection, signer, []solana.Instruction{depositInstruction})
		return err
	})
	group.Go(func() error {
		var err error
		batchTransactions, err = solanaWithdrawBatchPrepare(
			ctx, hinkal, chainID, token, depositedUtxos, feeStructure,
			rawEthereumAddress, recipientAmounts, pendingLeaves,
		)
		return err
	})
	if err := group.Wait(); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	scheduleID, err := web3.SolanaTransactCallRelayerBatch(
		ctx, chainID, batchTransactions, hashedEthereumAddress, txCompletionTime, "", "", depositTxHash,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}
