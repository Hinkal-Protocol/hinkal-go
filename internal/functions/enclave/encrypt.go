package enclave

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
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

func encryptUint8ArrayForEnclave(input []byte, publicKeyB64 string) (keyCiphertext, inputCiphertext string, err error) {
	var key [secretKeyBytes]byte
	if _, err = rand.Read(key[:]); err != nil {
		return "", "", err
	}
	keyCiphertext, err = asymmetricEncrypt(publicKeyB64, key[:])
	if err != nil {
		return "", "", err
	}
	inputCiphertext, err = symmetricEncrypt(&key, input)
	if err != nil {
		return "", "", err
	}
	return keyCiphertext, inputCiphertext, nil
}

func MakeHandshakeAndEncrypt(ctx context.Context, input []byte) (keyCiphertext, inputCiphertext string, err error) {
	pk, err := enclaveHandshakeService.GetPublicKey(ctx)
	if err != nil {
		return "", "", err
	}
	return encryptUint8ArrayForEnclave(input, pk)
}
