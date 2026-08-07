package enclave

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
)

func StoreAndGetSignatureFromEnclave(
	ctx context.Context,
	ethAddress string,
	authSignature string,
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) (string, error) {
	keyCiphertext, inputCiphertext, err := MakeHandshakeAndEncrypt(ctx, []byte(authSignature))
	if err != nil {
		return "", err
	}
	resp, err := api.StoreAndGetSignatureEnclaveCall(
		ctx,
		ethAddress,
		inputCiphertext,
		keyCiphertext,
		isSolanaLedger,
		txMessageForSolanaLedger,
	)
	if err != nil {
		return "", err
	}
	return resp.Signature, nil
}
