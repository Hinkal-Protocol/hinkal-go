package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errDepositOnChainUtxosLengthMismatch = errors.New("transactions: erc20Tokens and utxoAmounts length mismatch")
	errDepositOnChainUtxosEmptyToken     = errors.New("transactions: every token must deposit at least one UTXO")
	errDepositOnChainUtxosNoTimeStamp    = errors.New("transactions: deposit proof is missing the timestamp the on-chain UTXOs are stamped with")
	errDepositTxHashNotFound             = errors.New("transactions: deposit transaction hash not found")
)

type recipientUtxo struct {
	recipientAddress string
	utxo             *utxo.Utxo
}

type PreparedDepositOnChainUtxos struct {
	ChainID         int
	Erc20Tokens     []types.ERC20Token
	ExternalAddress string
	DepositProof    TransactProof
	DepositedUtxos  [][]*utxo.Utxo
	PendingLeaves   []string
	TotalAmounts    []*big.Int
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

func PrepareDepositOnChainUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Tokens []types.ERC20Token,
	utxoAmounts [][]*big.Int,
) (PreparedDepositOnChainUtxos, error) {
	if len(erc20Tokens) != len(utxoAmounts) {
		return PreparedDepositOnChainUtxos{}, errDepositOnChainUtxosLengthMismatch
	}
	for _, amounts := range utxoAmounts {
		if len(amounts) == 0 {
			return PreparedDepositOnChainUtxos{}, errDepositOnChainUtxosEmptyToken
		}
	}

	externalAddress, err := constants.DepositOnChainUtxosAddress(chainID)
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}
	if externalAddress == "" {
		return PreparedDepositOnChainUtxos{}, fmt.Errorf("transactions: DepositOnChainUtxos action is not deployed on chain %d", chainID)
	}

	erc20Addresses := make([]string, len(erc20Tokens))
	amountChanges := make([]*big.Int, len(erc20Tokens))
	onChainCreation := make([]bool, len(erc20Tokens))
	totalAmounts := make([]*big.Int, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
		amountChanges[i] = big.NewInt(0)
		onChainCreation[i] = true
		total := new(big.Int)
		for _, amount := range utxoAmounts[i] {
			total.Add(total, amount)
		}
		totalAmounts[i] = total
	}

	externalActionMetadata, err := web3.EncodeUint256Array2D(utxoAmounts)
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}

	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}
	ethereumAddress, err := utils.AddressToHexFormat(rawEthereumAddress)
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:                chainID,
		Erc20Addresses:         erc20Addresses,
		AmountChanges:          amountChanges,
		ExternalAddress:        externalAddress,
		ExternalActionID:       types.ExternalActionDepositOnChainUtxos,
		ExternalActionMetadata: []string{externalActionMetadata},
		OnChainCreation:        onChainCreation,
		OriginalSender:         ethereumAddress,
		ForceEmptyUtxos:        true,
		Submit:                 NewProofOnlySubmit(),
	})
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}
	depositProof := result.Proof

	if depositProof.CircomData.TimeStamp == "" {
		return PreparedDepositOnChainUtxos{}, errDepositOnChainUtxosNoTimeStamp
	}
	timeStamp, err := utils.ParseBigInt(depositProof.CircomData.TimeStamp)
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}
	stealthAddressStructure := depositProof.CircomData.StealthAddressStructure
	stealthAddress := utils.ToBeHex(stealthAddressStructure.StealthAddress)
	nullifyingKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return PreparedDepositOnChainUtxos{}, err
	}

	utxoIndex := new(big.Int)
	depositedUtxos := make([][]*utxo.Utxo, len(utxoAmounts))
	var pendingLeaves []string
	for tokenIndex, amounts := range utxoAmounts {
		depositedUtxos[tokenIndex] = make([]*utxo.Utxo, len(amounts))
		for i, amount := range amounts {
			note, err := utxo.NewUtxo(types.UtxoParams{
				Amount:            amount,
				Erc20TokenAddress: erc20Addresses[tokenIndex],
				TimeStamp:         new(big.Int).Add(timeStamp, utxoIndex).String(),
				NullifyingKey:     nullifyingKey,
				StealthAddress:    stealthAddress,
				H0:                &types.JubPoint{stealthAddressStructure.H0x, stealthAddressStructure.H0y},
				IsBlocked:         true,
			})
			if err != nil {
				return PreparedDepositOnChainUtxos{}, err
			}
			commitment, err := note.GetCommitment()
			if err != nil {
				return PreparedDepositOnChainUtxos{}, err
			}
			commitmentBig, err := utils.ParseBigInt(commitment)
			if err != nil {
				return PreparedDepositOnChainUtxos{}, err
			}
			depositedUtxos[tokenIndex][i] = note
			pendingLeaves = append(pendingLeaves, commitmentBig.String())
			utxoIndex.Add(utxoIndex, big.NewInt(1))
		}
	}

	return PreparedDepositOnChainUtxos{
		ChainID:         chainID,
		Erc20Tokens:     erc20Tokens,
		ExternalAddress: externalAddress,
		DepositProof:    depositProof,
		DepositedUtxos:  depositedUtxos,
		PendingLeaves:   pendingLeaves,
		TotalAmounts:    totalAmounts,
	}, nil
}

func SubmitDepositOnChainUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	prepared PreparedDepositOnChainUtxos,
	preEstimateGas bool,
) (string, error) {
	adapter, err := hinkal.GetProviderAdapter(&prepared.ChainID)
	if err != nil {
		return "", err
	}
	_, depositTxHash, err := web3.TransactCallDirect(ctx, adapter, prepared.ChainID, web3.TransactCallDirectParams{
		Amounts:          prepared.TotalAmounts,
		TokensToApprove:  prepared.Erc20Tokens,
		ZkCallData:       prepared.DepositProof.ZkCallData,
		CircomData:       prepared.DepositProof.CircomData,
		DimData:          prepared.DepositProof.DimData,
		ContractApproval: prepared.ExternalAddress,
		PreEstimateGas:   preEstimateGas,
	})
	if err != nil {
		return "", err
	}
	if depositTxHash == "" {
		return "", errDepositTxHashNotFound
	}
	return depositTxHash, nil
}
