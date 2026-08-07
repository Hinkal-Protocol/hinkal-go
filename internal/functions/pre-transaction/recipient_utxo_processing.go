package pretransaction

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func RecipientUtxoProcessing(
	recipientInfo bridging.PrivateRecipientInfo,
	outputUtxosArray [][]*utxo.Utxo,
	deltaChanges []*big.Int,
	timeStamp string,
) error {
	parts := strings.Split(recipientInfo.RecipientInfo, ",")
	if len(parts) < 6 {
		return fmt.Errorf("recipientUtxoProcessing: malformed recipient info %q", recipientInfo.RecipientInfo)
	}
	stealthAddress, h00, h01, encryptionKey := parts[0], parts[1], parts[2], parts[5]
	h00Big, err := utils.ParseBigInt(h00)
	if err != nil {
		return err
	}
	h01Big, err := utils.ParseBigInt(h01)
	if err != nil {
		return err
	}

	privateRecipientIndex := -1
	for i, outputUtxos := range outputUtxosArray {
		for _, u := range outputUtxos {
			if strings.EqualFold(u.Erc20TokenAddress, recipientInfo.Token.Erc20TokenAddress) {
				privateRecipientIndex = i
				break
			}
		}
		if privateRecipientIndex != -1 {
			break
		}
	}
	if privateRecipientIndex == -1 {
		return errors.New("recipientUtxoProcessing: private recipient index not found")
	}

	recipientUtxo, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            recipientInfo.Amount,
		Erc20TokenAddress: recipientInfo.Token.Erc20TokenAddress,
		TimeStamp:         timeStamp,
		H0:                &types.JubPoint{h00Big, h01Big},
		StealthAddress:    stealthAddress,
		EncryptionKey:     encryptionKey,
	})
	if err != nil {
		return err
	}

	for i, outputUtxos := range outputUtxosArray {
		if i == privateRecipientIndex {
			outputUtxosArray[i] = append(outputUtxos, recipientUtxo)
			deltaChanges[i] = new(big.Int).Add(deltaChanges[i], recipientInfo.Amount)
			continue
		}
		zeroUtxo, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            big.NewInt(0),
			Erc20TokenAddress: outputUtxos[0].Erc20TokenAddress,
			TimeStamp:         timeStamp,
			H0:                &types.JubPoint{h00Big, h01Big},
			StealthAddress:    stealthAddress,
			EncryptionKey:     encryptionKey,
		})
		if err != nil {
			return err
		}
		outputUtxosArray[i] = append(outputUtxos, zeroUtxo)
	}
	return nil
}
