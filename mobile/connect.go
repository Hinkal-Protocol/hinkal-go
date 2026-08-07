package mobile

import (
	"errors"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	hinkal "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/hinkal"
	mobileproviders "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/providers"
)

func (c *Client) completeConnect(adapter types.IProviderAdapter, chainID int) (string, error) {
	return hinkal.CompleteConnect(c.h, adapter, chainID)
}

func (c *Client) Connect(host HostWallet) (string, error) {
	if host == nil {
		return "", mobileerrors.ErrNilHostWallet
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	adapter := mobileproviders.NewWalletConnect(host)
	chainID := 0
	if cid := adapter.GetChainID(); cid != nil {
		chainID = *cid
	}
	return c.completeConnect(adapter, chainID)
}

func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return errors.Join(c.h.DisconnectFromConnector(), c.h.Destroy())
}
