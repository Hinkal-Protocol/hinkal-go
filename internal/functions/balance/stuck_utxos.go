package balance

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func GetInputUtxoAndBalanceOfStuckUtxos(ctx context.Context, p InputUtxoParams) ([]*utxo.Utxo, error) {
	ethereumAddress := p.EthAddress
	addressForHashing := ethereumAddress
	if constants.IsTronLike(p.ChainID) {
		hexAddr, err := utils.AddressToHexFormat(ethereumAddress)
		if err != nil {
			return nil, err
		}
		addressForHashing = hexAddr
	}
	hashedEthereumAddress := utils.HashEthereumAddress(addressForHashing)

	stuckParams := p
	stuckParams.UseBlockedUtxos = true
	inputUtxos, err := GetInputUtxoAndBalance(ctx, stuckParams)
	if err != nil {
		return nil, err
	}

	if len(inputUtxos) == 0 {
		return []*utxo.Utxo{}, nil
	}

	nullifiers := make([]string, 0, len(inputUtxos))
	for _, u := range inputUtxos {
		nul, err := u.GetNullifier()
		if err != nil {
			return nil, err
		}
		nullifiers = append(nullifiers, nul)
	}

	resp, err := api.GetScheduledTransactionsNullifierIndexes(ctx, api.ScheduledTransactionsNullifierIndexesRequest{
		HashedEthereumAddress: hashedEthereumAddress,
		Nullifiers:            nullifiers,
	})
	if err != nil {
		return nil, err
	}

	stuckUtxos := make([]*utxo.Utxo, 0, len(resp.Indexes))
	for _, index := range resp.Indexes {
		if index >= 0 && index < len(inputUtxos) {
			stuckUtxos = append(stuckUtxos, inputUtxos[index])
		}
	}
	return stuckUtxos, nil
}
