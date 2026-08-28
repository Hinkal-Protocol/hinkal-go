package enclave

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
)

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
	return resp.Signature, nil
}
