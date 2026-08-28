package enclave

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

const (
	secretKeyBytes = 32
	nonceBytes     = 24
)

type handshakeResponse struct {
	PublicKey string `json:"public_key"`
}

func makeHandshakeForPublicKey(ctx context.Context) (string, error) {
	var resp handshakeResponse
	if err := api.Get(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.Handshake, &resp); err != nil {
		return "", fmt.Errorf("enclave handshake: %w", err)
	}
	if resp.PublicKey == "" {
		return "", errors.New("enclave handshake: empty public key")
	}
	return resp.PublicKey, nil
}

func asymmetricEncrypt(publicKeyB64 string, content []byte) (string, error) {
	der, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode enclave public key: %w", err)
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return "", fmt.Errorf("parse enclave public key: %w", err)
	}
	rsaPub, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("enclave public key is not RSA")
	}
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, rsaPub, content, nil)
	if err != nil {
		return "", fmt.Errorf("rsa-oaep encrypt: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func symmetricEncrypt(key *[secretKeyBytes]byte, message []byte) (string, error) {
	var nonce [nonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	combined := make([]byte, 0, nonceBytes+len(message)+secretbox.Overhead)
	combined = append(combined, nonce[:]...)
	combined = secretbox.Seal(combined, message, &nonce, key)
	return base64.StdEncoding.EncodeToString(combined), nil
}

type Handshake struct {
	KeyCiphertext   string
	InputCiphertext string
	Key             [secretKeyBytes]byte
}

func encryptUint8ArrayForEnclave(input []byte, publicKeyB64 string) (Handshake, error) {
	var key [secretKeyBytes]byte
	if _, err := rand.Read(key[:]); err != nil {
		return Handshake{}, err
	}
	keyCiphertext, err := asymmetricEncrypt(publicKeyB64, key[:])
	if err != nil {
		return Handshake{}, err
	}
	inputCiphertext, err := symmetricEncrypt(&key, input)
	if err != nil {
		return Handshake{}, err
	}
	return Handshake{KeyCiphertext: keyCiphertext, InputCiphertext: inputCiphertext, Key: key}, nil
}

func MakeHandshakeAndEncrypt(ctx context.Context, input []byte) (Handshake, error) {
	pk, err := enclaveHandshakeService.GetPublicKey(ctx)
	if err != nil {
		return Handshake{}, err
	}
	return encryptUint8ArrayForEnclave(input, pk)
}

type sealedEnclaveResponse struct {
	Encrypted string `json:"encrypted"`
}

func OpenSealedResponse[T any](raw json.RawMessage, key [secretKeyBytes]byte) (T, error) {
	var dest T
	var sealed sealedEnclaveResponse
	if err := json.Unmarshal(raw, &sealed); err != nil || sealed.Encrypted == "" {
		return dest, errors.New("enclave answered in plaintext - refusing an unencrypted response")
	}
	combined, err := base64.StdEncoding.DecodeString(sealed.Encrypted)
	if err != nil {
		return dest, fmt.Errorf("decode sealed response: %w", err)
	}
	if len(combined) < nonceBytes {
		return dest, errors.New("sealed response too short")
	}
	var nonce [nonceBytes]byte
	copy(nonce[:], combined[:nonceBytes])
	plaintext, ok := secretbox.Open(nil, combined[nonceBytes:], &nonce, &key)
	if !ok {
		return dest, errors.New("failed to open sealed response")
	}
	if err := json.Unmarshal(plaintext, &dest); err != nil {
		return dest, fmt.Errorf("parse sealed response: %w", err)
	}
	return dest, nil
}
