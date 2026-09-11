package receivevault

import (
	"context"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	utxoimpl "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errNoH0 = errors.New("receive vault: H0 is not set")

// GetReceiveVaultParams derives the vault's params from the terms it was quoted with. Same terms,
// same params, same address: the derivation is the only record the protocol keeps.
func GetReceiveVaultParams(userKeys *cryptokeys.UserKeys, terms types.ReceiveVaultTerms) (types.ReceiveVaultParams, error) {
	nullifyingKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}
	seed, err := ReceiveVaultRandomizationSeed(nullifyingKey, terms)
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}
	randomization, err := cryptokeys.FindCorrectRandomization(seed, nullifyingKey)
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}

	utxo, err := utxoimpl.NewUtxo(types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: constants.ZeroAddress,
		NullifyingKey:     nullifyingKey,
		Randomization:     randomization,
		SpendingPublicKey: spendingKeyPair.PubSpendingBJJPoint[:],
	})
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}
	if utxo.H0 == nil {
		return types.ReceiveVaultParams{}, errNoH0
	}
	h1, err := cryptokeys.GetH1FromH0(*utxo.H0, nullifyingKey)
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}
	stealthAddress, err := utxo.GetStealthAddress()
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}
	stealthAddressBig, err := utils.ParseBigInt(stealthAddress)
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}

	var owner string
	if terms.Network == types.ReceiveVaultNetworkSolana {
		owner = GetReceiveVaultSolanaOwnerAddress(nullifyingKey, terms)
	} else {
		owner, err = GetReceiveVaultRecoveryAddress(nullifyingKey, terms)
		if err != nil {
			return types.ReceiveVaultParams{}, err
		}
	}
	version, err := parseVersion(terms.Version)
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}

	return types.ReceiveVaultParams{
		Recipient: types.StealthAddressStructure{
			H0x:            utxo.H0[0],
			H0y:            utxo.H0[1],
			H1x:            h1[0],
			H1y:            h1[1],
			StealthAddress: stealthAddressBig,
		},
		Owner:   owner,
		Version: version,
	}, nil
}

func BuildReceiveVaultRecord(
	ctx context.Context,
	userKeys *cryptokeys.UserKeys,
	terms types.ReceiveVaultTerms,
) (types.ReceiveVaultRecord, error) {
	params, err := GetReceiveVaultParams(userKeys, terms)
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}
	canonicalChainID, err := getReceiveVaultCanonicalChainID(terms.Network)
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}
	vaultAddress, err := GetReceiveVaultAddress(ctx, params, canonicalChainID)
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}
	return types.ReceiveVaultRecord{
		Index:        terms.Index,
		Network:      terms.Network,
		Version:      terms.Version,
		VaultAddress: vaultAddress,
	}, nil
}

// GetReceiveVaultEncryptionKey is the account-wide key the relayer encrypts settled outputs to.
func GetReceiveVaultEncryptionKey(userKeys *cryptokeys.UserKeys) (string, error) {
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	pair, err := cryptokeys.GetEncryptionKeyPair(shieldedPrivateKey)
	if err != nil {
		return "", err
	}
	return pair.PublicKey, nil
}
