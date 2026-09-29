package enclave

import (
	"context"
	"errors"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
)

type signatureData struct {
	EthAddress string `json:"ethAddress"`
	Signature  string `json:"signature"`
}

func StoreAndGetSignatureFromEnclave(
	ctx context.Context,
	ethAddress string,
	authSignature string,
) (string, error) {
	handshake, err := MakeHandshakeAndEncrypt(ctx, []byte(authSignature))
	if err != nil {
		return "", err
	}
	raw, err := api.StoreAndGetSignatureEnclaveCall(
		ctx,
		ethAddress,
		handshake.InputCiphertext,
		handshake.KeyCiphertext,
	)
	if err != nil {
		return "", err
	}
	resp, err := OpenSealedResponse[api.StoreAndGetSignatureResponse](raw, handshake.Key)
	if err != nil {
		return "", err
	}
	if resp.Data == "" || resp.EnclaveSignature == "" {
		return "", errors.New("enclave signature response is not signed")
	}
	verified, err := verifySignedEnclaveResponse[signatureData](resp.Data, resp.EnclaveSignature)
	if err != nil {
		return "", err
	}
	if verified.EthAddress != ethAddress {
		return "", errors.New("enclave signature response is for a different address")
	}
	return verified.Signature, nil
}
