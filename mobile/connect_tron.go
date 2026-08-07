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

func (c *Client) ConnectTron(host HostTronSigner, chainID int64) (string, error) {
	if host == nil {
		return "", mobileerrors.ErrNilHostSigner
	}
	return c.connectTron(mobileproviders.NewTronHostSigner(host), int(chainID))
}

func (c *Client) ConnectWithTronPrivateKey(privateKeyHex string, chainID int64) (string, error) {
	signer, err := signers.NewPrivateKeyTronSigner(privateKeyHex)
	if err != nil {
		return "", err
	}
	return c.connectTron(signer, int(chainID))
}

func (c *Client) connectTron(signer types.TronSigner, chainID int) (string, error) {
	if chainID == 0 {
		chainID = constants.CurrentTronChainID()
	}
	if !constants.IsTronLike(chainID) {
		return "", mobileerrors.ErrNotTronChain
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx := context.Background()
	adapter := providers.NewTronProviderAdapter(chainID)
	if err := adapter.InitConnector(ctx, signer); err != nil {
		return "", err
	}
	return c.completeConnect(adapter, chainID)
}
