package pretransaction

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func recipientAmountAt(recipientAddress string, recipientAmounts []*big.Int, index int) *big.Int {
	if recipientAddress == "" {
		return nil
	}
	if index < len(recipientAmounts) && recipientAmounts[index] != nil {
		return recipientAmounts[index]
	}
	return big.NewInt(0)
}

func BuildOutputUtxos(
	userKeys *cryptokeys.UserKeys,
	inputUtxosArray [][]*utxo.Utxo,
	amountChanges []*big.Int,
	recipientAddress string,
	recipientAmounts []*big.Int,
) ([][]*utxo.Utxo, error) {
	timeStamp := strconv.FormatInt(utils.GetCurrentTimeInSeconds(), 10)

	outputUtxosArray := make([][]*utxo.Utxo, len(inputUtxosArray))
	for i, inputUtxos := range inputUtxosArray {
		outputUtxos, err := OutputUtxoProcessing(
			userKeys,
			inputUtxos,
			amountChanges[i],
			timeStamp,
			true,
			recipientAddress,
			recipientAmountAt(recipientAddress, recipientAmounts, i),
		)
		if err != nil {
			return nil, err
		}
		outputUtxosArray[i] = outputUtxos
	}
	return outputUtxosArray, nil
}

func countTotalAmountInUtxos(utxos []*utxo.Utxo) *big.Int {
	total := new(big.Int)
	for _, u := range utxos {
		total.Add(total, u.Amount)
	}
	return total
}

func OutputUtxoProcessing(
	userKeys *cryptokeys.UserKeys,
	inputUtxos []*utxo.Utxo,
	amountChange *big.Int,
	timeStamp string,
	revertIfNegative bool,
	recipientAddress string,
	recipientAmountChange *big.Int,
) ([]*utxo.Utxo, error) {
	totalAmount := countTotalAmountInUtxos(inputUtxos)
	erc20TokenAddress := inputUtxos[0].Erc20TokenAddress
	mintAddress := inputUtxos[0].MintAddress

	if revertIfNegative && amountChange.Sign() < 0 && new(big.Int).Add(totalAmount, amountChange).Sign() < 0 {
		return nil, errors.New(errorhandling.ErrCodeInsufficientFundsToTransact)
	}

	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}

	changeUtxo, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            utils.BigintMax(new(big.Int).Add(totalAmount, amountChange), big.NewInt(0)),
		Erc20TokenAddress: erc20TokenAddress,
		MintAddress:       mintAddress,
		NullifyingKey:     shieldedPrivateKey,
		TimeStamp:         timeStamp,
		SpendingPublicKey: []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]},
	})
	if err != nil {
		return nil, err
	}
	outputUtxos := []*utxo.Utxo{changeUtxo}

	if recipientAddress != "" && recipientAmountChange != nil {
		parts := strings.Split(recipientAddress, ",")
		if len(parts) < 6 {
			return nil, fmt.Errorf("outputUtxoProcessing: malformed recipient address %q", recipientAddress)
		}
		stealthAddress, h00, h01, encryptionKey := parts[0], parts[1], parts[2], parts[5]
		h00Big, err := utils.ParseBigInt(h00)
		if err != nil {
			return nil, err
		}
		h01Big, err := utils.ParseBigInt(h01)
		if err != nil {
			return nil, err
		}
		recipientUtxo, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            recipientAmountChange,
			Erc20TokenAddress: erc20TokenAddress,
			MintAddress:       mintAddress,
			H0:                &types.JubPoint{h00Big, h01Big},
			StealthAddress:    stealthAddress,
			EncryptionKey:     encryptionKey,
			TimeStamp:         timeStamp,
		})
		if err != nil {
			return nil, err
		}
		outputUtxos = append(outputUtxos, recipientUtxo)
	}

	return outputUtxos, nil
}
