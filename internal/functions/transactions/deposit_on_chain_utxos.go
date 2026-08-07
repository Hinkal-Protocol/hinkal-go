package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/onchainutxos"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type recipientUtxo struct {
	recipientAddress string
	utxo             *utxo.Utxo
}

func depositAndWithdrawUtxoAmounts(recipientAmounts []*big.Int, feeStructure types.FeeStructure) ([]*big.Int, *big.Int) {
	utxoAmounts := make([]*big.Int, len(recipientAmounts))
	totalAmount := new(big.Int)
	for i, amount := range recipientAmounts {
		utxoAmounts[i] = new(big.Int).Add(amount, feeStructure.FlatFee)
		totalAmount.Add(totalAmount, utxoAmounts[i])
	}
	return utxoAmounts, totalAmount
}

func matchRecipientUtxos(recipientAddresses []string, utxoAmounts []*big.Int, depositedUtxos []*utxo.Utxo) ([]recipientUtxo, error) {
	available := append([]*utxo.Utxo{}, depositedUtxos...)
	out := make([]recipientUtxo, 0, len(recipientAddresses))
	for i, recipientAddress := range recipientAddresses {
		var matchIndex = -1
		for j, candidate := range available {
			if candidate.Amount.Cmp(utxoAmounts[i]) == 0 {
				matchIndex = j
				break
			}
		}
		if matchIndex < 0 {
			return nil, fmt.Errorf("could not find newly created UTXO with amount %s for recipient %s.", utxoAmounts[i], recipientAddress)
		}
		out = append(out, recipientUtxo{recipientAddress: recipientAddress, utxo: available[matchIndex]})
		available = append(available[:matchIndex], available[matchIndex+1:]...)
	}
	return out, nil
}

func HinkalDepositOnChainUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
	_ bool, // preEstimateGas, unused in ts also
) ([]recipientUtxo, string, string, error) {
	tokenAddress := erc20Token.Erc20TokenAddress
	utxoAmounts, _ := depositAndWithdrawUtxoAmounts(recipientAmounts, feeStructure)

	erc20Tokens := make([]types.ERC20Token, len(utxoAmounts))
	for i := range erc20Tokens {
		erc20Tokens[i] = erc20Token
	}

	statusResp, err := api.UpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeDeposit,
	})
	if err != nil {
		return nil, "", "", err
	}

	_, depositTxHash, err := HinkalProoflessDeposit(
		ctx, hinkal, erc20Tokens, utxoAmounts, nil, nil,
		true, types.ZeroProoflessFeeStructure(), "", false,
	)
	if err != nil {
		return nil, "", "", err
	}
	if depositTxHash == "" {
		return nil, "", "", errors.New("transactions: deposit transaction hash not found")
	}
	if _, err := hinkal.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		return nil, "", "", err
	}
	fetchClient, err := hinkal.GetFetchClient(chainID)
	if err != nil {
		return nil, "", "", err
	}
	receipt, err := web3.FetchTransactionReceiptWithRetry(ctx, fetchClient, depositTxHash)
	if err != nil {
		return nil, "", "", err
	}

	api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusResp.ID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseAfterDeposit,
		DepositTxHash:         depositTxHash,
	})

	depositedUtxos, err := onchainutxos.DecodeFromReceipt(receipt, hinkal.GetUserKeys(), chainID, tokenAddress)
	if err != nil {
		return nil, "", "", err
	}
	if len(depositedUtxos) == 0 {
		return nil, "", "", errNoUtxosInDepositTransaction
	}
	userDepositedUtxos, err := matchRecipientUtxos(recipientAddresses, utxoAmounts, depositedUtxos)
	if err != nil {
		return nil, "", "", err
	}
	return userDepositedUtxos, statusResp.ID, depositTxHash, nil
}
