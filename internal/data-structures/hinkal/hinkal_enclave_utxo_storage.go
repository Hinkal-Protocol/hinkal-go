package hinkal

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func (h *Hinkal) StoreClaimableKeyInEnclave(
	ctx context.Context,
	senderAddress string,
	recipientEthAddress string,
	shieldedPrivateKey string,
	chainID int,
	claimableSignature string,
) error {
	senderSignature, err := h.UserKeys.GetSignature()
	if err != nil {
		return err
	}
	return enclave.StoreClaimableKeyInEnclave(ctx, senderAddress, recipientEthAddress, shieldedPrivateKey, chainID, claimableSignature, senderSignature)
}

func (h *Hinkal) GetUtxosFromEnclave(
	ctx context.Context,
	ethAddress string,
	signature string,
	chainID int,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	return enclave.GetUtxosFromEnclave(ctx, ethAddress, signature, chainID)
}
