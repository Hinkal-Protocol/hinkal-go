package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errDepositAndWithdrawNoToken           = errors.New("transactions: depositAndWithdraw action: no token found")
	errDepositAndWithdrawOneToken          = errors.New("transactions: depositAndWithdraw supports one token")
	errRecipientAmountLengthMismatch       = errors.New("transactions: recipientAmounts and recipientAddresses length mismatch")
	errUserDepositedUtxosEmpty             = errors.New("userDepositedUtxos must not be empty")
	errNoUtxosInDepositTransaction         = errors.New("no UTXOs found in deposit transaction. Check contract address and ABI.")
	errDepositedUtxosMissingFromMerkle     = errors.New("timeout while waiting for deposited UTXOs to appear in Merkle tree.")
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

func areAllDepositedUtxosInLastLeaves(deposited []recipientUtxo, lastLeaves []*big.Int) (bool, error) {
	if len(deposited) == 0 {
		return true, nil
	}
	leafSet := make(map[string]struct{}, len(lastLeaves))
	for _, leaf := range lastLeaves {
		leafSet[leaf.String()] = struct{}{}
	}
	for _, depositedUtxo := range deposited {
		commitment, err := depositedUtxo.utxo.GetCommitment()
		if err != nil {
			return false, err
		}
		commitmentBig, err := utils.ParseBigInt(commitment)
		if err != nil {
			return false, err
		}
		if _, ok := leafSet[commitmentBig.String()]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func waitForDepositedUtxosInMerkleTree(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int, deposited []recipientUtxo) error {
	if len(deposited) == 0 {
		return nil
	}

	if hinkal.GenerateProofRemotely() && constants.IsEnclaveTxChain(chainID) {
		return nil
	}
	for attempt := 0; attempt < 60; attempt++ {
		if err := hinkal.ResetMerkle(ctx, chainID); err != nil {
			return err
		}
		var lastLeaves []*big.Int
		if tree := hinkal.MerkleTree(chainID); tree != nil {
			lastLeaves = tree.LastLeaves(20)
		}
		found, err := areAllDepositedUtxosInLastLeaves(deposited, lastLeaves)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errDepositedUtxosMissingFromMerkle
}

func hinkalWithdrawBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	hashedEthereumAddress string,
	statusID string,
	txCompletionTime *int,
) (string, error) {
	if len(userDepositedUtxos) == 0 {
		return "", errUserDepositedUtxosEmpty
	}
	tokenAddress := erc20Token.Erc20TokenAddress
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
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

				result, err := HinkalTransact(groupCtx, hinkal, HinkalTransactParams{
					ChainID:          chainID,
					Erc20Addresses:   []string{tokenAddress},
					AmountChanges:    []*big.Int{new(big.Int).Neg(utxoToWithdraw.Amount)},
					ExternalActionID: types.ExternalActionZero,
					ExternalAddress:  recipientAddressHex,
					FeeStructure:     &feeStructure,
					Relay:            relay,
					InputUtxos:       [][]*utxo.Utxo{{utxoToWithdraw}},
					UseBlockedUtxos:  true,
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
	scheduleID, err := web3.TransactCallRelayerBatch(ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "")
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
	tokenAddress := erc20Token.Erc20TokenAddress
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	ethereumAddress, err := utils.AddressToHexFormat(rawEthereumAddress)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(ethereumAddress)

	feeStructure, err := resolveDepositAndWithdrawFeeStructure(ctx, chainID, tokenAddress, feeStructureOverride)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	userDepositedUtxos, statusID, depositTxHash, err := HinkalDepositOnChainUtxos(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		recipientAmounts,
		recipientAddresses,
		feeStructure,
		hashedEthereumAddress,
		true,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if err := waitForDepositedUtxosInMerkleTree(ctx, hinkal, chainID, userDepositedUtxos); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	scheduleID, err := hinkalWithdrawBatch(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		userDepositedUtxos,
		feeStructure,
		ethereumAddress,
		hashedEthereumAddress,
		statusID,
		txCompletionTime,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}
