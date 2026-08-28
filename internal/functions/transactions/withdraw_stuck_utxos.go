package transactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const maxStuckUtxosPerTx = 6

var (
	errWithdrawStuckNoToken       = errors.New("transactions: withdrawStuckUtxos action: no token found")
	errWithdrawStuckTooManyTokens = errors.New("transactions: withdrawStuckUtxos supports one token")
	errWithdrawStuckNoUtxos       = errors.New("transactions: withdrawStuckUtxos no stuck UTXOs found")
	errNoStuckBalanceToWithdraw   = errors.New("No stuck balance found to withdraw")
	errInsufficientRecoveryFees   = errors.New("Insufficient funds to cover recovery fees")
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

	accounts := api.SolanaTransactAccounts{Recipient: recipientAddress}
	if !strings.EqualFold(mintAddresses[0], constants.SolanaNativeAddress) {
		accounts.Mint = mintAddresses[0]
	}

	result, err := SolanaTransact(ctx, hinkal, HinkalSolanaTransactParams{
		ChainID:         chainID,
		MintAddresses:   mintAddresses,
		AmountChanges:   amountChanges,
		RelayAddress:    relay,
		Recipient:       recipientAddress,
		Signer:          relay,
		FunctionName:    "transact",
		Accounts:        accounts,
		RelayerFee:      feeStructure.FlatFee,
		VariableRate:    feeStructure.VariableRate,
		UseBlockedUtxos: true,
		OnTxConfirm: func(notes SolanaTransactNotes) error {
			markNullifiersSpent(hinkal, chainID, [][]string{notes.InputNullifiers})
			return nil
		},
		Submit: SolanaTransactSubmit{Mode: SolanaSubmitModeRelayer},
	})
	if err != nil {
		return "", nil, err
	}
	return result.TxHash, amountToRecipient, nil
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

func stuckWithdrawViaEnclave(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipientAddress string,
) ([]string, error) {
	erc20Address := erc20Token.Erc20TokenAddress

	ethAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, erc20Address, []string{erc20Address}, types.ExternalActionTransact, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return nil, err
	}
	externalAddress, err := utils.AddressToHexFormat(recipientAddress)
	if err != nil {
		return nil, err
	}

	jobs, err := enclave.PrepareStuckWithdrawEnclaveCall(ctx, hinkal.GetUserKeys(), enclave.PrepareStuckWithdrawParams{
		ChainID:               chainID,
		Erc20Address:          erc20Address,
		ExternalAddress:       externalAddress,
		Relay:                 relay,
		FeeStructure:          feeStructure,
		HashedEthereumAddress: utils.HashEthereumAddress(ethAddress),
	})
	if err != nil {
		return nil, err
	}

	txHashes := make([]string, 0, len(jobs))
	for _, job := range jobs {
		signedMessageHash, err := utils.ParseBigInt(job.SignedMessageHash)
		if err != nil {
			return nil, err
		}
		sig, err := hinkal.GetUserKeys().SignEddsa(signedMessageHash)
		if err != nil {
			return nil, err
		}
		txHash, err := enclave.FinalizeTxEnclaveCallRelay(ctx, job.JobID, sig, chainID, enclave.FinalizeTxRelayExtra{})
		if err != nil {
			if len(txHashes) == 0 {
				return nil, err
			}
			log.Printf("stuck withdrawal partially submitted (%d/%d): %v", len(txHashes), len(jobs), err)
			return txHashes, nil
		}
		txHashes = append(txHashes, txHash)
	}
	return txHashes, nil
}

func stuckBalanceForToken(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	ethAddress string,
) *big.Int {
	balancesByChain, err := hinkal.GetStuckShieldedBalances(ctx, []int{chainID}, hinkal.GetUserKeys(), ethAddress)
	if err != nil {
		log.Printf("failed to read stuck balance after an empty stuck withdrawal: %v", err)
		return new(big.Int)
	}
	for _, balance := range balancesByChain[chainID] {
		if strings.EqualFold(balance.Token.Erc20TokenAddress, erc20Token.Erc20TokenAddress) {
			return balance.Balance
		}
	}
	return new(big.Int)
}

// Fees are quoted per nullifier count because the enclave picks the chunking.
func solanaStuckFeeStructures(
	ctx context.Context,
	chainID int,
	mintAddress, recipientAddress string,
) (map[string]types.FeeStructure, error) {
	feeStructures := make(map[string]types.FeeStructure, maxStuckUtxosPerTx)
	for nullifierCount := 1; nullifierCount <= maxStuckUtxosPerTx; nullifierCount++ {
		feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, mintAddress, []string{mintAddress}, types.ExternalActionTransact, nil, nil, &api.SolanaGasEstimateParams{
			MintTo:         mintAddress,
			Recipient:      recipientAddress,
			NullifierCount: nullifierCount,
		})
		if err != nil {
			return nil, err
		}
		feeStructures[strconv.Itoa(nullifierCount)] = feeStructure
	}
	return feeStructures, nil
}

func stuckWithdrawSolanaViaEnclave(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipientAddress string,
) ([]string, error) {
	mintAddress := erc20Token.Erc20TokenAddress

	ethAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return nil, err
	}
	feeStructures, err := solanaStuckFeeStructures(ctx, chainID, mintAddress, recipientAddress)
	if err != nil {
		return nil, err
	}

	accounts := api.SolanaTransactAccounts{Recipient: recipientAddress}
	if !strings.EqualFold(mintAddress, constants.SolanaNativeAddress) {
		accounts.Mint = mintAddress
	}
	accountsJSON, err := json.Marshal(accounts)
	if err != nil {
		return nil, err
	}

	jobs, err := enclave.PrepareSolanaStuckWithdrawEnclaveCall(ctx, hinkal.GetUserKeys(), types.PrepareSolanaStuckWithdrawParams{
		ChainID:               chainID,
		MintAddress:           mintAddress,
		Recipient:             recipientAddress,
		RelayAddress:          relay,
		Accounts:              accountsJSON,
		FeeStructures:         feeStructures,
		HashedEthereumAddress: utils.HashEthereumAddress(ethAddress),
	})
	if err != nil {
		return nil, err
	}

	txHashes := make([]string, 0, len(jobs))
	for _, job := range jobs {
		signedMessageHash, err := utils.ParseBigInt(job.SignedMessageHash)
		if err != nil {
			return nil, err
		}
		sig, err := hinkal.GetUserKeys().SignEddsa(signedMessageHash)
		if err != nil {
			return nil, err
		}
		txHash, err := enclave.FinalizeSolanaTxEnclaveCallRelay(ctx, job.JobID, sig, chainID, nil)
		if err != nil {
			return nil, err
		}
		txHashes = append(txHashes, txHash)
	}
	return txHashes, nil
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

	if hinkal.GenerateProofRemotely() {
		var txHashes []string
		if constants.IsSolanaLike(chainID) {
			txHashes, err = stuckWithdrawSolanaViaEnclave(ctx, hinkal, chainID, erc20Token, recipientAddress)
		} else {
			txHashes, err = stuckWithdrawViaEnclave(ctx, hinkal, chainID, erc20Token, recipientAddress)
		}
		if err != nil {
			return nil, err
		}
		if len(txHashes) == 0 {
			if stuckBalanceForToken(ctx, hinkal, chainID, erc20Token, ethAddress).Sign() > 0 {
				return nil, errInsufficientRecoveryFees
			}
			return nil, errNoStuckBalanceToWithdraw
		}
		return txHashes, nil
	}

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
