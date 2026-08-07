package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const maxStuckUtxosPerTx = 6

var (
	errWithdrawStuckNoToken       = errors.New("transactions: withdrawStuckUtxos action: no token found")
	errWithdrawStuckTooManyTokens = errors.New("transactions: withdrawStuckUtxos supports one token")
	errWithdrawStuckNoUtxos       = errors.New("transactions: withdrawStuckUtxos no stuck UTXOs found")
)

func countStuckUtxoAmount(utxos []*utxo.Utxo) *big.Int {
	total := new(big.Int)
	for _, u := range utxos {
		if u.Amount != nil {
			total.Add(total, u.Amount)
		}
	}
	return total
}

func positiveUtxosForToken(
	inputUtxos []*utxo.Utxo,
	chainID int,
	tokenAddress string,
) ([]*utxo.Utxo, error) {
	filtered := make([]*utxo.Utxo, 0, len(inputUtxos))
	for _, u := range inputUtxos {
		if u.Amount == nil || u.Amount.Sign() <= 0 {
			continue
		}
		candidateTokenAddress, err := u.GetTokenAddress(chainID)
		if err != nil {
			continue
		}
		if strings.EqualFold(candidateTokenAddress, tokenAddress) {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}

func sortStuckUtxos(filtered []*utxo.Utxo) []*utxo.Utxo {
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Amount.Cmp(filtered[j].Amount) > 0
	})
	return filtered
}

func stuckUtxoTokenKey(chainID int, tokenAddress string) string {
	if constants.IsSolanaLike(chainID) {
		return tokenAddress
	}
	return strings.ToLower(tokenAddress)
}

func positiveStuckUtxosForToken(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	tokenAddress string,
	ethAddress string,
) ([]*utxo.Utxo, error) {
	inputUtxosPerToken, err := balance.GetInputUtxoAndBalancePerToken(ctx, balance.InputUtxoParams{
		Hinkal:                hinkal,
		ChainID:               chainID,
		EthAddress:            ethAddress,
		AllowRemoteDecryption: hinkal.GenerateProofRemotely(),
		UseBlockedUtxos:       true,
	}, 2, false, nil, nil)
	if err != nil {
		return nil, err
	}
	inputUtxos := inputUtxosPerToken[stuckUtxoTokenKey(chainID, tokenAddress)]
	positiveUtxos, err := positiveUtxosForToken(inputUtxos, chainID, tokenAddress)
	if err != nil {
		return nil, err
	}
	return sortStuckUtxos(positiveUtxos), nil
}

func getSolanaStuckInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) ([]*utxo.Utxo, []*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, maxStuckUtxosPerTx, false, true)
	if err != nil {
		return nil, nil, err
	}
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxos, err := pretransaction.OutputUtxoProcessing(hinkal.GetUserKeys(), inputUtxosArray[0], amountChanges[0], timeStamp, true, "", nil)
	if err != nil {
		return nil, nil, err
	}
	return inputUtxosArray[0], outputUtxos, nil
}

func markNullifiersSpent(hinkal ihinkal.HinkalInternal, chainID int, inputNullifiers [][]string) {
	nullifiers := hinkal.Nullifiers(chainID)
	if nullifiers == nil {
		return
	}
	for _, perToken := range inputNullifiers {
		for _, nullifier := range perToken {
			if nullifier != "" && nullifier != "0" && nullifier != "0x" {
				nullifiers[nullifier] = struct{}{}
			}
		}
	}
}

func markInputUtxosSpent(hinkal ihinkal.HinkalInternal, chainID int, inputUtxosArray [][]*utxo.Utxo) {
	inputNullifiers := make([][]string, 0, len(inputUtxosArray))
	for _, inputUtxos := range inputUtxosArray {
		perToken := make([]string, 0, len(inputUtxos))
		for _, inputUtxo := range inputUtxos {
			if inputUtxo.Amount == nil || inputUtxo.Amount.Sign() == 0 {
				continue
			}
			nullifier, err := inputUtxo.GetNullifier()
			if err != nil {
				continue
			}
			perToken = append(perToken, nullifier)
		}
		inputNullifiers = append(inputNullifiers, perToken)
	}
	markNullifiersSpent(hinkal, chainID, inputNullifiers)
}

func withdrawSingleStuckToken(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	totalAmountInUtxos *big.Int,
	recipientAddressRaw string,
) (string, *big.Int, error) {
	tokenAddress := erc20Token.Erc20TokenAddress
	tokenAddresses := []string{tokenAddress}

	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, tokenAddress, tokenAddresses, types.ExternalActionTransact, nil, nil, nil)
	if err != nil {
		return "", nil, err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", nil, err
	}

	recipientAddress, err := utils.AddressToHexFormat(recipientAddressRaw)
	if err != nil {
		return "", nil, err
	}
	amountToRecipient := new(big.Int).Sub(totalAmountInUtxos, feeStructure.FlatFee)
	if amountToRecipient.Sign() <= 0 {
		return "", nil, fmt.Errorf("insufficient balance to cover fee. Balance: %s, Fee: %s", totalAmountInUtxos, feeStructure.FlatFee)
	}

	amountChanges := []*big.Int{new(big.Int).Neg(amountToRecipient)}
	if err := pretransaction.MergeWithFeeStructure(ctx, chainID, &tokenAddresses, &amountChanges, feeStructure); err != nil {
		return "", nil, err
	}

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:          chainID,
		Erc20Addresses:   tokenAddresses,
		AmountChanges:    amountChanges,
		ExternalActionID: types.ExternalActionZero,
		ExternalAddress:  recipientAddress,
		FeeStructure:     &feeStructure,
		Relay:            relay,
		UseBlockedUtxos:  true,
		OnTxConfirm: func(circomData types.CircomDataType) error {
			markNullifiersSpent(hinkal, chainID, circomData.InputNullifiers)
			return nil
		},
		Submit: NewRelayerSubmit(RelayerSubmit{}),
	})
	if err != nil {
		return "", nil, err
	}
	return result.TxHash, amountToRecipient, nil
}

func withdrawSingleStuckTokenSolana(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	totalAmountInUtxos *big.Int,
	recipientAddress string,
	nullifierCount int,
) (string, *big.Int, error) {
	mintAddresses := []string{erc20Token.Erc20TokenAddress}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, mintAddresses[0], mintAddresses, types.ExternalActionTransact, nil, nil, &api.SolanaGasEstimateParams{
		MintTo:         mintAddresses[0],
		Recipient:      recipientAddress,
		NullifierCount: nullifierCount,
	})
	if err != nil {
		return "", nil, err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", nil, err
	}

	amountToRecipient := new(big.Int).Sub(totalAmountInUtxos, feeStructure.FlatFee)
	if amountToRecipient.Sign() <= 0 {
		return "", nil, fmt.Errorf("insufficient balance to cover fee. Balance: %s, Fee: %s", totalAmountInUtxos, feeStructure.FlatFee)
	}

	amountChanges := []*big.Int{new(big.Int).Neg(amountToRecipient)}
	amountChanges[0] = new(big.Int).Sub(amountChanges[0], feeStructure.FlatFee)

	inputUtxos, outputUtxos, err := getSolanaStuckInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges)
	if err != nil {
		return "", nil, err
	}

	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return "", nil, err
	}
	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", nil, err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
	if err != nil {
		return "", nil, err
	}
	encryptedOutputBytes, encryptedOutputInts, err := solanaEncryptedOutputBytes(outputUtxos)
	if err != nil {
		return "", nil, err
	}
	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	outputUtxosArray := [][]*utxo.Utxo{outputUtxos}
	if err := pretransaction.EnsureAmountChanges(inputUtxosArray, outputUtxosArray, amountChanges); err != nil {
		return "", nil, err
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
		return "", nil, err
	}
	accounts := api.SolanaTransactAccounts{Recipient: recipientAddress}
	if !strings.EqualFold(mintAddresses[0], constants.SolanaNativeAddress) {
		accounts.Mint = mintAddresses[0]
	}

	txHash, err := web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
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
	})
	if err != nil {
		return "", nil, err
	}
	markInputUtxosSpent(hinkal, chainID, inputUtxosArray)
	return txHash, amountToRecipient, nil
}

func withdrawStuckTokenByChainID(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	totalAmount *big.Int,
	recipientAddress string,
	nullifierCount int,
) (string, *big.Int, error) {
	if totalAmount.Sign() == 0 {
		return "", nil, errWithdrawStuckNoUtxos
	}
	if constants.IsSolanaLike(chainID) {
		return withdrawSingleStuckTokenSolana(ctx, hinkal, chainID, erc20Token, totalAmount, recipientAddress, nullifierCount)
	}
	return withdrawSingleStuckToken(ctx, hinkal, chainID, erc20Token, totalAmount, recipientAddress)
}

func HinkalWithdrawStuckUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAddress string,
) ([]string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return nil, err
	}
	if len(erc20Tokens) == 0 {
		return nil, errWithdrawStuckNoToken
	}
	if len(erc20Tokens) > 1 {
		return nil, errWithdrawStuckTooManyTokens
	}

	ethAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	erc20Token := erc20Tokens[0]
	results := []string{}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, chainID); err != nil {
		return results, err
	}
	stuckUtxos, err := positiveStuckUtxosForToken(ctx, hinkal, chainID, erc20Token.Erc20TokenAddress, ethAddress)
	if err != nil {
		return results, err
	}
	for start := 0; start < len(stuckUtxos); start += maxStuckUtxosPerTx {
		end := start + maxStuckUtxosPerTx
		if end > len(stuckUtxos) {
			end = len(stuckUtxos)
		}
		chunk := stuckUtxos[start:end]
		totalAmount := countStuckUtxoAmount(chunk)
		txHash, _, err := withdrawStuckTokenByChainID(ctx, hinkal, chainID, erc20Token, totalAmount, recipientAddress, len(chunk))
		if err != nil {
			return results, err
		}
		results = append(results, txHash)
	}

	return results, nil
}
