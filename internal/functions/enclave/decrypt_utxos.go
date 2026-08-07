package enclave

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var mask128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

func GetInputUtxosEnclave(ctx context.Context, chainID int, uk *cryptokeys.UserKeys) (utxos []*utxo.Utxo, encryptedOutputs []*types.EncryptedOutputWithSign, lastOutput string, err error) {
	shieldedPrivateKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, nil, "", err
	}
	sk, pk, err := cryptokeys.EncryptionKeyPair(shieldedPrivateKey)
	if err != nil {
		return nil, nil, "", err
	}

	spendingPair, err := uk.GetSpendingKeyPair()
	if err != nil {
		return nil, nil, "", err
	}
	spendingPublicKey := []*big.Int{spendingPair.PubSpendingBJJPoint[0], spendingPair.PubSpendingBJJPoint[1]}

	data, err := packEnclaveKeys(pk, sk, shieldedPrivateKey)
	if err != nil {
		return nil, nil, "", err
	}

	keyCiphertext, inputCiphertext, err := MakeHandshakeAndEncrypt(ctx, data)
	if err != nil {
		return nil, nil, "", err
	}

	resp, err := api.DecryptUtxoEnclaveCall(ctx, chainID, keyCiphertext, inputCiphertext)
	if err != nil {
		return nil, nil, "", err
	}

	utxos = make([]*utxo.Utxo, 0, len(resp.Utxos))
	for _, item := range resp.Utxos {
		u, err := deserializeEnclaveUtxo(item, shieldedPrivateKey, spendingPublicKey, chainID)
		if err != nil {
			return nil, nil, "", err
		}
		utxos = append(utxos, u)
	}

	return utxos, resp.EncryptedOutputs, resp.LastOutput, nil
}

func packEnclaveKeys(pk, sk [32]byte, shieldedPrivateKey string) ([]byte, error) {
	shieldedBig, err := utils.ParseBigInt(shieldedPrivateKey)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 96)
	copy(data[0:32], pk[:])
	copy(data[32:64], sk[:])
	shieldedBig.FillBytes(data[64:96])
	return data, nil
}

func deserializeEnclaveUtxo(item types.UtxoParams, shieldedPrivateKey string, spendingPublicKey []*big.Int, chainID int) (*utxo.Utxo, error) {
	item.NullifyingKey = shieldedPrivateKey
	item.SpendingPublicKey = spendingPublicKey

	if constants.IsSolanaLike(chainID) {
		compressed, base58Mint, err := normalizeSolanaMint(item.MintAddress)
		if err != nil {
			return nil, err
		}
		item.Erc20TokenAddress = compressed
		item.MintAddress = base58Mint
	}

	return utxo.NewUtxo(item)
}

func normalizeSolanaMint(mintHex string) (compressedHex, base58Mint string, err error) {
	clean := common.FromHex(mintHex)
	if len(clean) != 32 {
		return "", "", fmt.Errorf("invalid mint address: expected 32 bytes, got %d", len(clean))
	}
	base58Mint = base58.Encode(clean)

	mintBig := new(big.Int).SetBytes(clean)
	part1 := new(big.Int).Rsh(mintBig, 128)
	part2 := new(big.Int).And(mintBig, mask128)
	compressed, err := crypto.PoseidonBig(part1, part2)
	if err != nil {
		return "", "", err
	}
	return utils.ToBeHex(compressed), base58Mint, nil
}
