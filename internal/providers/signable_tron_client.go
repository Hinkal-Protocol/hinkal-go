package providers

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// SignableTronClient pairs a Tron signer with the HTTPS wallet client. It satisfies tron.TronClient,
// so transactions are built, signed locally, and broadcast over TLS — never plaintext gRPC.
type SignableTronClient struct {
	walletClient *tron.WalletClient
	signer       types.TronSigner
	address      string
}

func newSignableTronClient(walletClient *tron.WalletClient, signer types.TronSigner, address string) *SignableTronClient {
	return &SignableTronClient{walletClient: walletClient, signer: signer, address: address}
}

func (c *SignableTronClient) WalletClient() *tron.WalletClient {
	return c.walletClient
}

func (c *SignableTronClient) GetAddress() string {
	return c.address
}

func (c *SignableTronClient) SignAndBroadcast(ctx context.Context, tx *tron.Transaction) (string, error) {
	return tron.SignAndBroadcast(ctx, c.walletClient, c.signer.SignTxHash, tx)
}
