package utxo

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func ensure0x(value string) string {
	if strings.HasPrefix(value, "0x") {
		return value
	}
	return "0x" + value
}

func encryptionRecipients(encryptionKeyHex string) ([]*[32]byte, error) {
	keys := []string{encryptionKeyHex}
	if constants.EnclavePubkey != "" {
		keys = append(keys, constants.EnclavePubkey)
	}
	recipients := make([]*[32]byte, len(keys))
	for i, k := range keys {
		pkBytes := common.FromHex(k)
		if len(pkBytes) != 32 {
			return nil, errors.New("utxo: encryption public key is not 32 bytes")
		}
		var pk [32]byte
		copy(pk[:], pkBytes)
		recipients[i] = &pk
	}
	return recipients, nil
}

func EncryptEncryptionKeyAndStealthAddress(encryptionKeyHex string, stealthAddress *big.Int) ([]byte, error) {
	buf := append([]byte(ensure0x(encryptionKeyHex)), []byte(utils.ToBeHex(stealthAddress))...)
	recipients, err := encryptionRecipients(encryptionKeyHex)
	if err != nil {
		return nil, err
	}
	encrypted, err := cryptokeys.EncryptSealedKeys(buf, recipients)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func EncryptUtxo(u *Utxo) ([]byte, error) {
	stealth, err := u.GetStealthAddress()
	if err != nil {
		return nil, err
	}
	ts, err := utils.ParseBigInt(u.TimeStamp)
	if err != nil {
		return nil, err
	}

	encKeyHex, err := u.GetEncryptionKey()
	if err != nil {
		return nil, err
	}

	h0x, h0y := big.NewInt(0), big.NewInt(0)
	if u.H0 != nil {
		h0x, h0y = (*u.H0)[0], (*u.H0)[1]
	}

	// converting data to Uint8Array; all data is in hex format
	parts := [][]byte{
		[]byte(utils.ToBeHex(u.Amount)),
		[]byte(ensure0x(u.Erc20TokenAddress)),
		[]byte(ensure0x(stealth)),
		[]byte(utils.ToBeHex(ts)),
		[]byte(utils.ToBeHex(h0x)),  // point H0[0]
		[]byte(utils.ToBeHex(h0y)),  // point H0[1]
		[]byte(ensure0x(encKeyHex)), // owner id for enclave mapping (deterministic per user)
	}
	if u.MintAddress != "" {
		mintBytes, err := base58.Decode(u.MintAddress)
		if err != nil {
			return nil, err
		}
		parts = append(parts, []byte("0x"+hex.EncodeToString(mintBytes)))
	}

	var buf []byte
	for _, p := range parts {
		buf = append(buf, p...)
	}

	recipients, err := encryptionRecipients(encKeyHex)
	if err != nil {
		return nil, err
	}

	// encrypting with encryptionPublicKey (+ enclave)
	encrypted, err := cryptokeys.EncryptSealedKeys(buf, recipients)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}
