package mobile

import (
	"strings"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/signers"
)

func (c *Client) ConnectWithPrivateKey(privateKeyHex string, chainID64 int64) (string, error) {
	chainID := int(chainID64)
	if chainID == 0 {
		chainID = constants.ChainIDs.EthMainnet
	}
	if !constants.IsEvmChain(chainID) {
		return "", mobileerrors.ErrNotEVMChain
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	signer, err := signers.NewPrivateKeyEVMSigner(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", err
	}
	adapter, err := providers.NewEthersProviderAdapter()
	if err != nil {
		return "", err
	}
	adapter.InitSigner(signer)
	cid := chainID
	if err := adapter.Init(&cid); err != nil {
		return "", err
	}

	return c.completeConnect(adapter, chainID)
}
