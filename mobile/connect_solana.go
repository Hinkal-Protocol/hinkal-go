package mobile

import (
	"context"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/signers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	mobileproviders "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/providers"
)

func (c *Client) ConnectSolana(host HostSolanaSigner, chainID int64) (string, error) {
	if host == nil {
		return "", mobileerrors.ErrNilHostSigner
	}
	return c.connectSolana(mobileproviders.NewSolanaHostSigner(host), int(chainID))
}

func (c *Client) ConnectWithSolanaPrivateKey(privateKeyBase58 string, chainID int64) (string, error) {
	signer, err := signers.NewPrivateKeySolanaSigner(privateKeyBase58)
	if err != nil {
		return "", err
	}
	return c.connectSolana(signer, int(chainID))
}

func (c *Client) connectSolana(signer types.SolanaSigner, chainID int) (string, error) {
	if chainID == 0 {
		chainID = constants.CurrentSolanaChainID
	}
	if !constants.IsSolanaLike(chainID) {
		return "", mobileerrors.ErrNotSolanaChain
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx := context.Background()
	pub, err := signer.GetPublicKey(ctx)
	if err != nil {
		return "", err
	}
	adapter, err := providers.NewSolanaProviderAdapter(chainID, pub.String())
	if err != nil {
		return "", err
	}
	adapter.InitConnector(signer)
	return c.completeConnect(adapter, chainID)
}
