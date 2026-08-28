package balance

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func buildZeroUtxo(chainID int, erc20Address string, shieldedPrivateKey string, spendingPublicKey []*big.Int) (*utxo.Utxo, error) {
	mintAddress := ""
	modifiedErc20TokenAddress := erc20Address
	if constants.IsSolanaLike(chainID) {
		mintAddress = erc20Address
		formatted, err := solanautils.FormatMintAddress(erc20Address)
		if err != nil {
			return nil, err
		}
		modifiedErc20TokenAddress = formatted.CompressedAddress
	}
	return utxo.NewUtxo(types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: modifiedErc20TokenAddress,
		MintAddress:       mintAddress,
		NullifyingKey:     shieldedPrivateKey,
		SpendingPublicKey: spendingPublicKey,
	})
}

func buildZeroUtxos(chainID int, erc20Address string, shieldedPrivateKey string, spendingPublicKey []*big.Int, count int) ([]*utxo.Utxo, error) {
	utxos := make([]*utxo.Utxo, 0, count)
	for i := 0; i < count; i++ {
		padUtxo, err := buildZeroUtxo(chainID, erc20Address, shieldedPrivateKey, spendingPublicKey)
		if err != nil {
			return nil, err
		}
		utxos = append(utxos, padUtxo)
	}
	return utxos, nil
}

func AddPaddingToUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	maxInput int,
	forceEmptyUtxos bool,
	useBlockedUtxos bool,
	onChainCreation []bool,
) ([][]*utxo.Utxo, error) {
	if maxInput == 0 {
		maxInput = 6
	}

	userKeys := hinkal.GetUserKeys()
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}

	if len(erc20Addresses) == 0 {
		return [][]*utxo.Utxo{}, nil
	}

	ethAddress := ""
	if useBlockedUtxos {
		ethAddress, err = hinkal.GetEthereumAddressByChain(ctx, chainID)
		if err != nil {
			return nil, err
		}
	}

	inputUtxosPerToken, err := GetInputUtxoAndBalancePerToken(ctx, InputUtxoParams{
		Hinkal:                hinkal,
		ChainID:               chainID,
		EthAddress:            ethAddress,
		AllowRemoteDecryption: hinkal.GenerateProofRemotely(),
		UseBlockedUtxos:       useBlockedUtxos,
	}, 2, false, nil, erc20Addresses)
	if err != nil {
		return nil, err
	}

	inputUtxosArrayToBePadded := make([][]*utxo.Utxo, 0, len(erc20Addresses))
	maxUtxoNum := 0
	for i := 0; i < len(erc20Addresses); i++ {
		switch {
		case forceEmptyUtxos:
			inputUtxosArrayToBePadded = append(inputUtxosArrayToBePadded, []*utxo.Utxo{})
		case i < len(onChainCreation) && onChainCreation[i]:
			zeroUtxos, zeroErr := buildZeroUtxos(chainID, erc20Addresses[i], shieldedPrivateKey, spendingPublicKey, 2)
			if zeroErr != nil {
				return nil, zeroErr
			}
			if len(zeroUtxos) > maxUtxoNum {
				maxUtxoNum = len(zeroUtxos)
			}
			inputUtxosArrayToBePadded = append(inputUtxosArrayToBePadded, zeroUtxos)
		default:
			key := encodeTokenWithID(chainID, erc20Addresses[i])
			inputUtxos := inputUtxosPerToken[key]
			if len(inputUtxos) > maxUtxoNum {
				maxUtxoNum = len(inputUtxos)
			}
			inputUtxosArrayToBePadded = append(inputUtxosArrayToBePadded, inputUtxos)
		}
	}

	if maxUtxoNum == 2 {
		return inputUtxosArrayToBePadded, nil
	}

	inputUtxosArrayPadded := make([][]*utxo.Utxo, 0, len(inputUtxosArrayToBePadded))
	for i, utxos := range inputUtxosArrayToBePadded {
		if len(utxos) > maxInput {
			firstSixUtxos := utxos[:maxInput]
			firstSixAmount := new(big.Int)
			for _, u := range firstSixUtxos {
				firstSixAmount.Add(firstSixAmount, u.Amount)
			}
			if amountChanges[i].Sign() < 0 && firstSixAmount.Cmp(new(big.Int).Neg(amountChanges[i])) < 0 {
				if err := overLimitError(ctx, firstSixAmount, erc20Addresses[i], chainID); err != nil {
					return nil, err
				}
			}
			inputUtxosArrayPadded = append(inputUtxosArrayPadded, firstSixUtxos)
		} else {
			diff := maxInput - len(utxos)
			pads, err := buildZeroUtxos(chainID, erc20Addresses[i], shieldedPrivateKey, spendingPublicKey, diff)
			if err != nil {
				return nil, err
			}
			tempUtxosStorage := append(append([]*utxo.Utxo{}, utxos...), pads...)
			inputUtxosArrayPadded = append(inputUtxosArrayPadded, tempUtxosStorage)
		}
	}

	return inputUtxosArrayPadded, nil
}

func overLimitError(ctx context.Context, firstSixAmount *big.Int, erc20Address string, chainID int) error {
	token, err := web3.GetErc20TokenFromAPI(ctx, chainID, erc20Address)
	if err != nil || token == nil {
		return &errorhandling.ErrorWithAmount{Amount: 0, Message: errorhandling.ErrCodeUtxoLimitations}
	}
	hintPrecision := 2
	if token.Decimals == 18 {
		hintPrecision = 6
	}
	amountStr, err := web3.GetAmountWithPrecision(firstSixAmount, *token, hintPrecision)
	if err != nil {
		return err
	}
	amountInToken, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("%s. Please try again with %s %s, including gas fees.",
		errorhandling.ErrCodeUtxoLimitations, strconv.FormatFloat(amountInToken, 'g', -1, 64), token.Symbol)
	return &errorhandling.ErrorWithAmount{Amount: amountInToken, Message: message}
}
