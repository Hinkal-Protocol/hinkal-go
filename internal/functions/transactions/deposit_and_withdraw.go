package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errDepositAndWithdrawNoToken           = errors.New("transactions: depositAndWithdraw action: no token found")
	errDepositAndWithdrawOneToken          = errors.New("transactions: depositAndWithdraw supports one token")
	errRecipientAmountLengthMismatch       = errors.New("transactions: recipientAmounts and recipientAddresses length mismatch")
	errUserDepositedUtxosEmpty             = errors.New("userDepositedUtxos must not be empty")
	errDepositProofMissingEncryptedOutputs = errors.New("transactions: deposit proof is missing encryptedOutputs for the deposited UTXOs")
	errDepositAndWithdrawUnsupportedSolana = errors.New("transactions: use HinkalSolanaDepositAndWithdraw for Solana chains")
)

func resolveDepositAndWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	feeStructureOverride *types.FeeStructure,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return normalizeFeeStructure(*feeStructureOverride), nil
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tokenAddress,
		[]string{tokenAddress},
		types.ExternalActionTransact,
		nil,
		constants.PaySendVariableRate(),
		nil,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return normalizeFeeStructure(feeStructure), nil
}

func validateDepositAndWithdrawArgs(
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
) error {
	if len(erc20Tokens) == 0 {
		return errDepositAndWithdrawNoToken
	}
	if len(erc20Tokens) > 1 {
		return errDepositAndWithdrawOneToken
	}
	if len(recipientAmounts) == 0 {
		return errAmountsEmpty
	}
	if len(recipientAmounts) != len(recipientAddresses) {
		return errRecipientAmountLengthMismatch
	}
	for _, amount := range recipientAmounts {
		if amount == nil || amount.Sign() <= 0 {
			return errAmountNotPositive
		}
	}
	return nil
}

func hinkalWithdrawBatchPrepare(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	pendingLeaves []string,
) ([]web3.TransactCallRelayerBatchItem, error) {
	if len(userDepositedUtxos) == 0 {
		return nil, errUserDepositedUtxosEmpty
	}
	tokenAddress := erc20Token.Erc20TokenAddress
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return nil, err
	}
	transactions := make([]web3.TransactCallRelayerBatchItem, 0, len(userDepositedUtxos))

	generateProofRemotely := hinkal.GenerateProofRemotely()
	batchSize := 1
	if generateProofRemotely && !constants.IsTronLike(chainID) {
		batchSize = proofBatchSize(generateProofRemotely)
	}
	for start := 0; start < len(userDepositedUtxos); start += batchSize {
		end := start + batchSize
		if end > len(userDepositedUtxos) {
			end = len(userDepositedUtxos)
		}
		batchItems := userDepositedUtxos[start:end]
		batchTransactions := make([]web3.TransactCallRelayerBatchItem, len(batchItems))
		group, groupCtx := errgroup.WithContext(ctx)

		for batchIndex, item := range batchItems {
			batchIndex, item := batchIndex, item
			group.Go(func() error {
				recipientAddressHex, err := utils.AddressToHexFormat(item.recipientAddress)
				if err != nil {
					return err
				}
				utxoToWithdraw := item.utxo
				if !strings.EqualFold(utxoToWithdraw.Erc20TokenAddress, tokenAddress) {
					utxoToWithdraw, err = utxo.CreateFrom(utxoToWithdraw, types.UtxoParams{Erc20TokenAddress: tokenAddress})
					if err != nil {
						return err
					}
				}

				inputUtxos := [][]*utxo.Utxo{{utxoToWithdraw}}
				var speculative *types.SpeculativeTreeParams
				if pendingLeaves != nil {
					speculativeInputs, err := pretransaction.ToSpeculativeUtxos(inputUtxos)
					if err != nil {
						return err
					}
					speculative = &types.SpeculativeTreeParams{PendingLeaves: pendingLeaves, InputUtxos: speculativeInputs}
				}

				result, err := HinkalTransact(groupCtx, hinkal, HinkalTransactParams{
					ChainID:          chainID,
					Erc20Addresses:   []string{tokenAddress},
					AmountChanges:    []*big.Int{new(big.Int).Neg(utxoToWithdraw.Amount)},
					ExternalActionID: types.ExternalActionZero,
					ExternalAddress:  recipientAddressHex,
					FeeStructure:     &feeStructure,
					Relay:            relay,
					InputUtxos:       inputUtxos,
					UseBlockedUtxos:  true,
					Speculative:      speculative,
					SkipLock:         true,
					Submit:           NewProofOnlySubmit(),
				})
				if err != nil {
					return err
				}
				proof := result.Proof

				adminData := pretransaction.ConstructAdminData(
					types.AdminPublicToPublicSend,
					chainID,
					[]string{tokenAddress},
					[]*big.Int{new(big.Int).Neg(utxoToWithdraw.Amount)},
					ethereumAddress,
					nil,
				)

				batchTransactions[batchIndex] = web3.TransactCallRelayerBatchItem{
					ZkCallData:               proof.ZkCallData,
					DimData:                  proof.DimData,
					CircomData:               proof.CircomData,
					CommitmentValidationData: proof.CommitmentValidationData,
					RecipientAddress:         item.recipientAddress,
					AdminData:                adminData,
					TronProofSignature:       proof.TronProofSignature,
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

func HinkalWithdrawBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	hashedEthereumAddress string,
	txCompletionTime *int,
) (string, error) {
	transactions, err := hinkalWithdrawBatchPrepare(
		ctx, hinkal, chainID, erc20Token, userDepositedUtxos, feeStructure, ethereumAddress, nil,
	)
	if err != nil {
		return "", err
	}
	return web3.TransactCallRelayerBatch(ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "", "")
}

type depositAndWithdrawContext struct {
	chainID               int
	ethereumAddress       string
	hashedEthereumAddress string
	feeStructure          types.FeeStructure
	txCompletionTime      *int
	preEstimateGas        bool
}

func evmDepositAndWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	depositContext depositAndWithdrawContext,
) (types.DepositAndSendExtendedResult, error) {
	chainID := depositContext.chainID
	utxoAmounts, _ := depositAndWithdrawUtxoAmounts(recipientAmounts, depositContext.feeStructure)

	preparedDeposit, err := PrepareDepositOnChainUtxos(ctx, hinkal, chainID, []types.ERC20Token{erc20Token}, [][]*big.Int{utxoAmounts})
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	recipientUtxos := make([]recipientUtxo, len(recipientAddresses))
	for i, note := range preparedDeposit.DepositedUtxos[0] {
		recipientUtxos[i] = recipientUtxo{recipientAddress: recipientAddresses[i], utxo: note}
	}

	var depositTxHash string
	var batchTransactions []web3.TransactCallRelayerBatchItem
	// Plain errgroup, not WithContext: a failing proof must not cancel a deposit already in flight.
	var group errgroup.Group
	group.Go(func() error {
		var err error
		depositTxHash, err = SubmitDepositOnChainUtxos(ctx, hinkal, preparedDeposit, depositContext.preEstimateGas)
		return err
	})
	group.Go(func() error {
		var err error
		batchTransactions, err = hinkalWithdrawBatchPrepare(
			ctx, hinkal, chainID, erc20Token, recipientUtxos, depositContext.feeStructure,
			depositContext.ethereumAddress, preparedDeposit.PendingLeaves,
		)
		return err
	})
	if err := group.Wait(); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	scheduleID, err := web3.TransactCallRelayerBatch(
		ctx, chainID, batchTransactions, depositContext.hashedEthereumAddress,
		depositContext.txCompletionTime, "", "", depositTxHash,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}

func tronDepositAndWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	depositContext depositAndWithdrawContext,
) (types.DepositAndSendExtendedResult, error) {
	chainID := depositContext.chainID
	utxoAmounts, totalDepositAmount := depositAndWithdrawUtxoAmounts(recipientAmounts, depositContext.feeStructure)

	depositResult, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:            chainID,
		Erc20Addresses:     []string{erc20Token.Erc20TokenAddress},
		AmountChanges:      []*big.Int{totalDepositAmount},
		ExternalAddress:    depositContext.ethereumAddress,
		SelfOutputAmounts:  utxoAmounts,
		CreateBlockedUtxos: true,
		ForceEmptyUtxos:    true,
		Submit:             NewProofOnlySubmit(),
	})
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	depositProof := depositResult.Proof

	if len(depositProof.CircomData.EncryptedOutputs) == 0 ||
		len(depositProof.CircomData.EncryptedOutputs[0]) != len(utxoAmounts) {
		return types.DepositAndSendExtendedResult{}, errDepositProofMissingEncryptedOutputs
	}
	recipientUtxos := make([]recipientUtxo, len(recipientAddresses))
	for i, encryptedOutput := range depositProof.CircomData.EncryptedOutputs[0] {
		note, err := balance.DecryptUtxoHex(encryptedOutput, hinkal.GetUserKeys())
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
		recipientUtxos[i] = recipientUtxo{recipientAddress: recipientAddresses[i], utxo: note}
	}

	var pendingLeaves []string
	for _, perToken := range depositProof.CircomData.OutCommitments {
		for _, commitment := range perToken {
			commitmentBig, err := utils.ParseBigInt(commitment)
			if err != nil {
				return types.DepositAndSendExtendedResult{}, err
			}
			if commitmentBig.Sign() == 0 {
				continue
			}
			pendingLeaves = append(pendingLeaves, commitmentBig.String())
		}
	}

	var depositTxHash string
	var batchTransactions []web3.TransactCallRelayerBatchItem
	// Plain errgroup, not WithContext: a failing proof must not cancel a deposit already in flight.
	var group errgroup.Group
	group.Go(func() error {
		client, err := hinkal.GetTronWeb()
		if err != nil {
			return err
		}
		tron.SwapTronBCoordinate(&depositProof.ZkCallData)
		depositTxHash, err = tron.TransactCallDirectTron(ctx, client, chainID, tron.TransactCallDirectTronParams{
			Amounts:                   []*big.Int{totalDepositAmount},
			TokensToApprove:           []types.ERC20Token{erc20Token},
			ZkCallData:                depositProof.ZkCallData,
			CircomData:                depositProof.CircomData,
			DimData:                   depositProof.DimData,
			PreEstimateGas:            depositContext.preEstimateGas,
			PrecomputedProofSignature: depositProof.TronProofSignature,
		})
		if err != nil {
			return err
		}
		if depositTxHash == "" {
			return errDepositTxHashNotFound
		}
		return nil
	})
	group.Go(func() error {
		var err error
		batchTransactions, err = hinkalWithdrawBatchPrepare(
			ctx, hinkal, chainID, erc20Token, recipientUtxos, depositContext.feeStructure,
			depositContext.ethereumAddress, pendingLeaves,
		)
		return err
	})
	if err := group.Wait(); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	scheduleID, err := web3.TransactCallRelayerBatch(
		ctx, chainID, batchTransactions, depositContext.hashedEthereumAddress,
		depositContext.txCompletionTime, "", "", depositTxHash,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}

func HinkalDepositAndWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if constants.IsSolanaLike(chainID) {
		return types.DepositAndSendExtendedResult{}, errDepositAndWithdrawUnsupportedSolana
	}
	if err := validateDepositAndWithdrawArgs(erc20Tokens, recipientAmounts, recipientAddresses); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	erc20Token := erc20Tokens[0]
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	ethereumAddress, err := utils.AddressToHexFormat(rawEthereumAddress)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	feeStructure, err := resolveDepositAndWithdrawFeeStructure(ctx, chainID, erc20Token.Erc20TokenAddress, feeStructureOverride)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	depositContext := depositAndWithdrawContext{
		chainID:               chainID,
		ethereumAddress:       ethereumAddress,
		hashedEthereumAddress: utils.HashEthereumAddress(ethereumAddress),
		feeStructure:          feeStructure,
		txCompletionTime:      txCompletionTime,
		preEstimateGas:        preEstimateGas,
	}

	if !constants.IsTronLike(chainID) {
		return evmDepositAndWithdraw(ctx, hinkal, erc20Token, recipientAmounts, recipientAddresses, depositContext)
	}

	if len(recipientAmounts) > constants.MaxTronSelfOutputs {
		return types.DepositAndSendExtendedResult{}, fmt.Errorf(
			"transactions: Tron supports at most %d recipients per deposit - no circuit emits more outputs",
			constants.MaxTronSelfOutputs,
		)
	}
	return tronDepositAndWithdraw(ctx, hinkal, erc20Token, recipientAmounts, recipientAddresses, depositContext)
}
