package cryptokeys

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	sealBytes        = 48
	secretKeyBytes   = 32
	secretNonceBytes = 24
	sealedKeyLen     = sealBytes + secretKeyBytes
)

func DecryptBoxSeal(ciphertext []byte, pk, sk *[32]byte) ([]byte, error) {
	msg, ok := box.OpenAnonymous(nil, ciphertext, pk, sk)
	if !ok {
		return nil, errors.New("cryptokeys: box seal open failed")
	}
	return msg, nil
}

func DecryptSealedKeys(packed []byte, pk, sk *[32]byte, recipientIndex int) ([]byte, error) {
	if len(packed) < 1+secretNonceBytes {
		return nil, errors.New("cryptokeys: sealed keys too short")
	}
	count := int(packed[0])
	off := 1
	var nonce [secretNonceBytes]byte
	copy(nonce[:], packed[off:off+secretNonceBytes])
	off += secretNonceBytes

	if recipientIndex < 0 || recipientIndex >= count {
		return nil, errors.New("cryptokeys: recipient index out of range")
	}
	if len(packed) < off+count*sealedKeyLen {
		return nil, errors.New("cryptokeys: sealed keys truncated")
	}
	sealedStart := off + recipientIndex*sealedKeyLen
	sealedKey := packed[sealedStart : sealedStart+sealedKeyLen]
	ciphertext := packed[off+count*sealedKeyLen:]

	contentKey, ok := box.OpenAnonymous(nil, sealedKey, pk, sk)
	if !ok || len(contentKey) != secretKeyBytes {
		return nil, errors.New("cryptokeys: sealed content key open failed")
	}
	var key [32]byte
	copy(key[:], contentKey)
	msg, ok := secretbox.Open(nil, ciphertext, &nonce, &key)
	if !ok {
		return nil, errors.New("cryptokeys: secretbox open failed")
	}
	return msg, nil
}

func EncryptSealedKeys(msg []byte, recipientPubKeys []*[32]byte) ([]byte, error) {
	var contentKey [32]byte
	if _, err := rand.Read(contentKey[:]); err != nil {
		return nil, err
	}
	var nonce [secretNonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	ciphertext := secretbox.Seal(nil, msg, &nonce, &contentKey)

	packed := make([]byte, 0, 1+secretNonceBytes+len(recipientPubKeys)*sealedKeyLen+len(ciphertext))
	packed = append(packed, byte(len(recipientPubKeys)))
	packed = append(packed, nonce[:]...)
	for _, pk := range recipientPubKeys {
		sealed, err := box.SealAnonymous(nil, contentKey[:], pk, rand.Reader)
		if err != nil {
			return nil, err
		}
		packed = append(packed, sealed...)
	}
	packed = append(packed, ciphertext...)
	return packed, nil
}
