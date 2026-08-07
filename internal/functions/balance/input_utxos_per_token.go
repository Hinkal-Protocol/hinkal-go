package balance

import (
	"context"
	"math/big"
	"sort"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func encodeTokenWithID(chainID int, erc20TokenAddress string) string {
	if constants.IsSolanaLike(chainID) {
		return erc20TokenAddress
	}
	return strings.ToLower(erc20TokenAddress)
}

func GetInputUtxoAndBalancePerToken(
	ctx context.Context,
	p InputUtxoParams,
	minInput int,
	sliceIfMore6 bool,
	tokensWithID []string,
	ensuredTokensWithID []string,
) (map[string][]*utxo.Utxo, error) {
	userKeys := p.Hinkal.GetUserKeys()
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	var inputUtxos []*utxo.Utxo
	if p.UseBlockedUtxos {
		inputUtxos, err = GetInputUtxoAndBalanceOfStuckUtxos(ctx, p)
	} else {
		inputUtxos, err = GetInputUtxoAndBalance(ctx, p)
	}
	if err != nil {
		return nil, err
	}

	inputUtxosPerToken := map[string][]*utxo.Utxo{}
	var keyOrder []string
	addKey := func(key string) {
		if _, ok := inputUtxosPerToken[key]; !ok {
			inputUtxosPerToken[key] = nil
			keyOrder = append(keyOrder, key)
		}
	}

	for _, item := range inputUtxos {
		tokenAddress, err := item.GetTokenAddress(p.ChainID)
		if err != nil {
			continue
		}
		if tokensWithID != nil && !anyTokenAddressMatches(tokensWithID, tokenAddress) {
			continue
		}
		key := encodeTokenWithID(p.ChainID, tokenAddress)
		addKey(key)
		inputUtxosPerToken[key] = append(inputUtxosPerToken[key], item)
	}

	for _, item := range ensuredTokensWithID {
		addKey(encodeTokenWithID(p.ChainID, item))
	}

	for _, key := range keyOrder {
		erc20TokenAddress := key
		mintAddress := ""
		modifiedErc20TokenAddress := strings.ToLower(erc20TokenAddress)
		if constants.IsSolanaLike(p.ChainID) {
			mintAddress = erc20TokenAddress
			formatted, err := solanautils.FormatMintAddress(erc20TokenAddress)
			if err != nil {
				return nil, err
			}
			modifiedErc20TokenAddress = strings.ToLower(formatted.CompressedAddress)
		}
		padded, err := sortAndPadUtxos(inputUtxosPerToken[key], minInput, sliceIfMore6, shieldedPrivateKey, spendingPublicKey, modifiedErc20TokenAddress, mintAddress)
		if err != nil {
			return nil, err
		}
		inputUtxosPerToken[key] = padded
	}

	return inputUtxosPerToken, nil
}

func sortAndPadUtxos(
	inputUtxos []*utxo.Utxo,
	minInput int,
	sliceIfMore6 bool,
	shieldedPrivateKey string,
	spendingPublicKey []*big.Int,
	erc20TokenAddress string,
	mintAddress string,
) ([]*utxo.Utxo, error) {
	sort.SliceStable(inputUtxos, func(i, j int) bool {
		return inputUtxos[i].Amount.Cmp(inputUtxos[j].Amount) > 0
	})

	for len(inputUtxos) < minInput || (len(inputUtxos) > minInput && len(inputUtxos) < 6) {
		padUtxo, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            big.NewInt(0),
			Erc20TokenAddress: erc20TokenAddress,
			MintAddress:       mintAddress,
			NullifyingKey:     shieldedPrivateKey,
			SpendingPublicKey: spendingPublicKey,
		})
		if err != nil {
			return nil, err
		}
		inputUtxos = append(inputUtxos, padUtxo)

		if sliceIfMore6 {
			for len(inputUtxos) > 6 {
				inputUtxos = inputUtxos[:len(inputUtxos)-1]
			}
		}
	}

	return inputUtxos, nil
}

func anyTokenAddressMatches(tokensWithID []string, tokenAddress string) bool {
	for _, t := range tokensWithID {
		if strings.EqualFold(t, tokenAddress) {
			return true
		}
	}
	return false
}
