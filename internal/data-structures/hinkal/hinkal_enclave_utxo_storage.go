package hinkal

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func (h *Hinkal) StoreUtxoInEnclave(
	ctx context.Context,
	senderAddress string,
	recipientEthAddress string,
	u *utxo.Utxo,
	chainID int,
	claimableSignature string,
) error {
	return enclave.StoreUtxoInEnclave(ctx, senderAddress, recipientEthAddress, u, chainID, claimableSignature)
}

func (h *Hinkal) GetUtxosFromEnclave(
	ctx context.Context,
	ethAddress string,
	signature string,
	chainID int,
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	return enclave.GetUtxosFromEnclave(ctx, ethAddress, signature, chainID, isSolanaLedger, txMessageForSolanaLedger)
}
