package privatewallet

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"

	gethcommon "github.com/ethereum/go-ethereum/common"
	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type DerivedTempWallet struct {
	Address    string
	PrivateKey string
}

// This function is just derived from the EVM private key to the Solana private key, Not all wallets have the same private key format.
func SolanaPrivateKeyFromEVMPrivateKey(evmPrivateKey string) (string, error) {
	seed := gethcommon.FromHex(evmPrivateKey)
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("privatewallet: expected a %d byte EVM private key, got %d", ed25519.SeedSize, len(seed))
	}
	return solana.PrivateKey(ed25519.NewKeyFromSeed(seed)).String(), nil
}

func SolanaSignerAddressFromPrivateKey(solanaPrivateKey string) (string, error) {
	key, err := solana.PrivateKeyFromBase58(solanaPrivateKey)
	if err != nil {
		return "", fmt.Errorf("privatewallet: invalid solana private key: %w", err)
	}
	return key.PublicKey().String(), nil
}

func DeriveTempWalletFromEVMPrivateKey(chainID int, evmPrivateKey string) (DerivedTempWallet, error) {
	if constants.IsSolanaLike(chainID) {
		solanaPrivateKey, err := SolanaPrivateKeyFromEVMPrivateKey(evmPrivateKey)
		if err != nil {
			return DerivedTempWallet{}, err
		}
		address, err := SolanaSignerAddressFromPrivateKey(solanaPrivateKey)
		if err != nil {
			return DerivedTempWallet{}, err
		}
		return DerivedTempWallet{Address: address, PrivateKey: solanaPrivateKey}, nil
	}

	evmAddress, err := SignerAddressFromPrivateKey(chainID, evmPrivateKey)
	if err != nil {
		return DerivedTempWallet{}, err
	}
	if constants.IsTronLike(chainID) {
		tronAddress, err := utils.EVMHexToTronBase58Address(evmAddress)
		if err != nil {
			return DerivedTempWallet{}, err
		}
		return DerivedTempWallet{Address: tronAddress, PrivateKey: evmPrivateKey}, nil
	}
	return DerivedTempWallet{Address: evmAddress, PrivateKey: evmPrivateKey}, nil
}

func DeriveTempWallet(userKeys *cryptokeys.UserKeys, chainID int, nonce *big.Int, isLegacy bool) (DerivedTempWallet, error) {
	evmPrivateKey, err := tempWalletEVMPrivateKey(userKeys, nonce, isLegacy)
	if err != nil {
		return DerivedTempWallet{}, err
	}
	return DeriveTempWalletFromEVMPrivateKey(chainID, evmPrivateKey)
}

func tempWalletEVMPrivateKey(userKeys *cryptokeys.UserKeys, nonce *big.Int, isLegacy bool) (string, error) {
	if isLegacy {
		return userKeys.GetLegacySignerPrivateKeyFromNonce(nonce)
	}
	return userKeys.GetSignerPrivateKeyFromNonce(nonce)
}

func BuildTempSubAccount(nonce *big.Int, chainID int, wallet DerivedTempWallet) types.TemporarySubAccount {
	subAccount := types.TemporarySubAccount{Index: int(nonce.Int64())}
	if constants.IsSolanaLike(chainID) {
		subAccount.SolanaAddress = wallet.Address
		subAccount.SolanaPrivateKey = wallet.PrivateKey
		return subAccount
	}
	subAccount.EthAddress = wallet.Address
	subAccount.PrivateKey = wallet.PrivateKey
	return subAccount
}

// BuildTempSubAccountForChain derives the temp wallet for a nonce; isLegacy comes from the server's
// nonce entry (nk-derived wallets), newly issued nonces are never legacy.
func BuildTempSubAccountForChain(userKeys *cryptokeys.UserKeys, chainID int, nonce *big.Int, isLegacy bool) (types.TemporarySubAccount, error) {
	evmPrivateKey, err := tempWalletEVMPrivateKey(userKeys, nonce, isLegacy)
	if err != nil {
		return types.TemporarySubAccount{}, err
	}
	wallet, err := DeriveTempWalletFromEVMPrivateKey(chainID, evmPrivateKey)
	if err != nil {
		return types.TemporarySubAccount{}, err
	}
	return BuildTempSubAccount(nonce, chainID, wallet), nil
}

func AddressFromSubAccount(chainID int, subAccount types.TemporarySubAccount) (string, error) {
	address := subAccount.EthAddress
	if constants.IsSolanaLike(chainID) {
		address = subAccount.SolanaAddress
	}
	if address == "" {
		return "", errors.New("privatewallet: sub-account has no address for this chain")
	}
	return address, nil
}

func PrivateKeyFromSubAccount(chainID int, subAccount types.TemporarySubAccount) (string, error) {
	privateKey := subAccount.PrivateKey
	if constants.IsSolanaLike(chainID) {
		privateKey = subAccount.SolanaPrivateKey
		if privateKey == "" {
			privateKey = subAccount.PrivateKey
		}
	}
	if privateKey == "" {
		return "", errors.New("privatewallet: sub-account has no private key for this chain")
	}
	return privateKey, nil
}
