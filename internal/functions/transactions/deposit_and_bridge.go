package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errDepositAndBridgeNoRecipients      = errors.New("no recipients to bridge")
	errDepositAndBridgeUnsupportedSource = errors.New("transactions: depositAndBridge supports EVM source chains only")
	errDepositAndBridgeMissingNativeUtxo = errors.New("transactions: native fee UTXO is required for Lifi bridge")
)

type bridgeRecipientUtxo struct {
	types.BridgeRecipient
	utxo       *utxo.Utxo
	nativeUtxo *utxo.Utxo
}

func nativeFeeValue(quote types.BridgeQuote) *big.Int {
	if quote.NativeFee == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(quote.NativeFee)
}

func validateDepositAndBridgeArgs(erc20Tokens []types.ERC20Token, recipients []types.BridgeRecipient) error {
	if len(erc20Tokens) == 0 {
		return errDepositAndWithdrawNoToken
	}
	if len(erc20Tokens) > 1 {
		return errDepositAndWithdrawOneToken
	}
	if len(recipients) == 0 {
		return errDepositAndBridgeNoRecipients
	}
	for _, recipient := range recipients {
		if recipient.BridgeAmount == nil || recipient.BridgeAmount.Sign() <= 0 {
			return errAmountNotPositive
		}
		if recipient.RecipientAddress == "" || recipient.TemporarySubAccount.EthAddress == "" || recipient.TemporarySubAccount.PrivateKey == "" {
			return errorhandling.ErrRecipientFormatIncorrect
		}
		if recipient.Quote.Calldata == "" {
			return errors.New("transactions: bridge quote calldata is required")
		}
	}
	return nil
}

func resolveDepositAndBridgeFeeStructure(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	recipients []types.BridgeRecipient,
	feeStructureOverride *types.FeeStructure,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return normalizeFeeStructure(*feeStructureOverride), nil
	}
	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return types.FeeStructure{}, err
	}
	firstRecipient := recipients[0]
	sampleOps, err := privatewallet.CreateLifiBridgeOps(
		chainID,
		firstRecipient.TemporarySubAccount.EthAddress,
		lifiRouterAddress,
		tokenAddress,
		firstRecipient.BridgeAmount,
		firstRecipient.BridgeAmount,
		firstRecipient.Quote,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	sampleCalls := make([]types.CallInfo, len(sampleOps))
	for i, op := range sampleOps {
		call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, firstRecipient.TemporarySubAccount.EthAddress, chainID)
		if err != nil {
			return types.FeeStructure{}, err
		}
		sampleCalls[i] = call
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tokenAddress,
		[]string{tokenAddress},
		types.ExternalActionEmporium,
		sampleCalls,
		constants.PaySendVariableRate(),
		nil,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return normalizeFeeStructure(feeStructure), nil
}

func zeroDepositFeeStructure(feeStructure types.FeeStructure) types.FeeStructure {
	feeToken := feeStructure.FeeToken
	if feeToken == "" {
		feeToken = constants.DefaultFeeToken
	}
	return types.FeeStructure{
		FeeToken:     feeToken,
		FlatFee:      big.NewInt(0),
		VariableRate: big.NewInt(0),
	}
}

func depositedBridgeRecipients(recipients []types.BridgeRecipient, mainDeposits, nativeDeposits []recipientUtxo) []bridgeRecipientUtxo {
	out := make([]bridgeRecipientUtxo, len(recipients))
	for i, recipient := range recipients {
		out[i] = bridgeRecipientUtxo{
			BridgeRecipient: recipient,
			utxo:            mainDeposits[i].utxo,
		}
		if len(nativeDeposits) > i {
			out[i].nativeUtxo = nativeDeposits[i].utxo
		}
	}
	return out
}

func bridgeInputUtxoForToken(source *utxo.Utxo, tokenAddress string) (*utxo.Utxo, error) {
	if strings.EqualFold(source.Erc20TokenAddress, tokenAddress) {
		return source, nil
	}
	return utxo.CreateFrom(source, types.UtxoParams{Erc20TokenAddress: tokenAddress})
}

func hinkalBridgeBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipients []bridgeRecipientUtxo,
	feeStructure types.FeeStructure,
	ethereumAddress string,
	hashedEthereumAddress string,
	statusID string,
	txCompletionTime *int,
) (string, error) {
	if len(recipients) == 0 {
		return "", errDepositAndBridgeNoRecipients
	}
	tokenAddress := erc20Token.Erc20TokenAddress
	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return "", err
	}
	if contractData.EmporiumAddress == "" {
		return "", errors.New("no Emporium Address")
	}
	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return "", err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	fetchClient, err := hinkal.GetFetchClient(chainID)
	if err != nil {
		return "", err
	}

	transactions := make([]web3.TransactCallRelayerBatchItem, 0, len(recipients))
	generateProofRemotely := hinkal.GenerateProofRemotely()
	useEnclave := generateProofRemotely && constants.IsEnclaveTxChain(chainID)
	batchSize := proofBatchSize(generateProofRemotely)
	for start := 0; start < len(recipients); start += batchSize {
		end := start + batchSize
		if end > len(recipients) {
			end = len(recipients)
		}
		batchRecipients := recipients[start:end]
		batchTransactions := make([]web3.TransactCallRelayerBatchItem, len(batchRecipients))
		group, groupCtx := errgroup.WithContext(ctx)

		for batchIndex, recipient := range batchRecipients {
			batchIndex, recipient := batchIndex, recipient
			group.Go(func() error {
				if _, err := api.AddTemporaryWalletNonce(groupCtx, chainID, hashedEthereumAddress, recipient.TemporarySubAccount.Index, types.TemporaryWalletRecoveryDestinationPublic); err != nil {
					return err
				}

				utxoToBridge, err := bridgeInputUtxoForToken(recipient.utxo, tokenAddress)
				if err != nil {
					return err
				}
				inputUtxosArray := [][]*utxo.Utxo{{utxoToBridge}}
				onChainCreation := []bool{false}

				needsNativeFee := nativeFeeValue(recipient.Quote).Sign() > 0 && !strings.EqualFold(tokenAddress, constants.ZeroAddress)
				if needsNativeFee {
					if recipient.nativeUtxo == nil {
						return errDepositAndBridgeMissingNativeUtxo
					}
					nativeUtxo, err := bridgeInputUtxoForToken(recipient.nativeUtxo, constants.ZeroAddress)
					if err != nil {
						return err
					}
					inputUtxosArray = append(inputUtxosArray, []*utxo.Utxo{nativeUtxo})
					onChainCreation = append(onChainCreation, false)
				}

				ops, err := privatewallet.CreateLifiBridgeOps(
					chainID,
					recipient.TemporarySubAccount.EthAddress,
					lifiRouterAddress,
					tokenAddress,
					utxoToBridge.Amount,
					recipient.BridgeAmount,
					recipient.Quote,
				)
				if err != nil {
					return err
				}
				var proof TransactProof
				var authorizationData *types.AuthorizationData
				proofGroup, proofCtx := errgroup.WithContext(groupCtx)
				proofGroup.Go(func() error {
					bridgeErc20Addresses := []string{tokenAddress}
					bridgeAmountChanges := []*big.Int{new(big.Int).Neg(utxoToBridge.Amount)}
					if needsNativeFee {
						bridgeErc20Addresses = append(bridgeErc20Addresses, constants.ZeroAddress)
						bridgeAmountChanges = append(bridgeAmountChanges, new(big.Int).Neg(inputUtxosArray[1][0].Amount))
					}

					messageSeed, err := utils.RandomBigInt(31)
					if err != nil {
						return err
					}

					var externalActionMetadata []string
					if useEnclave {
						emporiumMessage, err := crypto.PoseidonBig(messageSeed)
						if err != nil {
							return err
						}
						signerAddress, err := privatewallet.SignerAddressFromPrivateKey(chainID, recipient.TemporarySubAccount.PrivateKey)
						if err != nil {
							return err
						}
						encoded, err := privatewallet.EncodeEmporiumMetadata(chainID, contractData.EmporiumAddress, recipient.TemporarySubAccount.PrivateKey, ops, emporiumMessage, signerAddress)
						if err != nil {
							return err
						}
						externalActionMetadata = []string{encoded}
					}

					result, err := HinkalTransact(proofCtx, hinkal, HinkalTransactParams{
						ChainID:                chainID,
						Erc20Addresses:         bridgeErc20Addresses,
						AmountChanges:          bridgeAmountChanges,
						ExternalActionID:       types.ExternalActionEmporium,
						ExternalAddress:        contractData.EmporiumAddress,
						ExternalActionMetadata: externalActionMetadata,
						FeeStructure:           &feeStructure,
						Relay:                  relay,
						OnChainCreation:        onChainCreation,
						MessageSeed:            messageSeed,
						InputUtxos:             inputUtxosArray,
						UseBlockedUtxos:        true,
						SubAccountPrivateKey:   recipient.TemporarySubAccount.PrivateKey,
						EmporiumOps:            ops,
						Submit:                 NewProofOnlySubmit(),
					})
					if err != nil {
						return err
					}
					proof = result.Proof
					return nil
				})
				proofGroup.Go(func() error {
					var err error
					authorizationData, err = privatewallet.GetAuthorizationDataIfNeeded(proofCtx, fetchClient, chainID, recipient.TemporarySubAccount.PrivateKey)
					return err
				})
				if err := proofGroup.Wait(); err != nil {
					return err
				}

				adminData := pretransaction.ConstructAdminData(
					types.AdminPublicToPublicBridgeSend,
					chainID,
					[]string{tokenAddress},
					[]*big.Int{new(big.Int).Neg(utxoToBridge.Amount)},
					ethereumAddress,
					nil,
				)

				batchTransactions[batchIndex] = web3.TransactCallRelayerBatchItem{
					ZkCallData:               proof.ZkCallData,
					DimData:                  proof.DimData,
					CircomData:               proof.CircomData,
					CommitmentValidationData: proof.CommitmentValidationData,
					AuthorizationData:        authorizationData,
					RecipientAddress:         recipient.RecipientAddress,
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

func HinkalDepositAndBridge(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipients []types.BridgeRecipient,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if constants.IsSolanaLike(chainID) || constants.IsTronLike(chainID) {
		return types.DepositAndSendExtendedResult{}, errDepositAndBridgeUnsupportedSource
	}
	if err := validateDepositAndBridgeArgs(erc20Tokens, recipients); err != nil {
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

	feeStructure, err := resolveDepositAndBridgeFeeStructure(ctx, chainID, tokenAddress, recipients, feeStructureOverride)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	zeroFeeStructure := zeroDepositFeeStructure(feeStructure)
	isNativeInput := strings.EqualFold(tokenAddress, constants.ZeroAddress)

	recipientAddresses := make([]string, len(recipients))
	mainTokenNetAmounts := make([]*big.Int, len(recipients))
	nativeFeeAmounts := make([]*big.Int, len(recipients))
	totalNativeFee := big.NewInt(0)
	for i, recipient := range recipients {
		recipientAddresses[i] = recipient.RecipientAddress
		nativeFeeAmounts[i] = nativeFeeValue(recipient.Quote)
		totalNativeFee.Add(totalNativeFee, nativeFeeAmounts[i])

		netAmount := new(big.Int).Set(recipient.BridgeAmount)
		if isNativeInput {
			netAmount.Add(netAmount, nativeFeeAmounts[i])
		}
		mainTokenNetAmounts[i] = fees.CalculateGrossAmountWithFee(netAmount, feeStructure)
	}

	mainDeposits, statusID, depositTxHash, err := HinkalDepositOnChainUtxos(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		mainTokenNetAmounts,
		recipientAddresses,
		zeroFeeStructure,
		hashedEthereumAddress,
		true,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	var nativeDeposits []recipientUtxo
	needsNativeDeposit := totalNativeFee.Sign() > 0 && !isNativeInput
	if needsNativeDeposit {
		nativeToken, err := web3.GetErc20TokenFromAPI(ctx, chainID, constants.ZeroAddress)
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
		if nativeToken == nil {
			return types.DepositAndSendExtendedResult{}, errors.New("no native token")
		}
		nativeFeeStructure := types.FeeStructure{FeeToken: feeStructure.FeeToken, FlatFee: big.NewInt(0), VariableRate: feeStructure.VariableRate}
		grossedNativeFees := make([]*big.Int, len(nativeFeeAmounts))
		for i, amount := range nativeFeeAmounts {
			grossedNativeFees[i] = fees.CalculateGrossAmountWithFee(amount, nativeFeeStructure)
		}
		nativeDeposits, _, _, err = HinkalDepositOnChainUtxos(
			ctx,
			hinkal,
			chainID,
			*nativeToken,
			grossedNativeFees,
			recipientAddresses,
			zeroFeeStructure,
			hashedEthereumAddress,
			true,
		)
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
	}

	allDeposits := append([]recipientUtxo{}, mainDeposits...)
	allDeposits = append(allDeposits, nativeDeposits...)
	if err := waitForDepositedUtxosInMerkleTree(ctx, hinkal, chainID, allDeposits); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	scheduleID, err := hinkalBridgeBatch(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		depositedBridgeRecipients(recipients, mainDeposits, nativeDeposits),
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
