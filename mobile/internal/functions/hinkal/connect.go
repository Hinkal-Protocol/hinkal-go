package hinkal

import (
	"context"

	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"
)

func initKeys(ctx context.Context, h *core.Hinkal) error {
	if _, err := h.GetShieldedPublicKey(); err == nil {
		return nil
	}
	return h.InitUserKeys(ctx, types.LoginMessageModeProtocol)
}

func CompleteConnect(h *core.Hinkal, adapter types.IProviderAdapter, chainID int) (string, error) {
	ctx := context.Background()
	if err := h.InitProviderAdapter(ctx, adapter); err != nil {
		return "", err
	}
	if err := initKeys(ctx, h); err != nil {
		return "", err
	}
	return connectResultJSON(ctx, h, chainID)
}

func connectResultJSON(ctx context.Context, h *core.Hinkal, chainID int) (string, error) {
	shielded, err := h.GetShieldedPublicKey()
	if err != nil {
		return "", err
	}
	eth := ""
	if chainID != 0 {
		eth, err = h.GetEthereumAddressByChain(ctx, chainID)
	} else {
		eth, err = h.GetEthereumAddress(ctx)
	}
	if err != nil {
		return "", err
	}
	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		return "", err
	}
	return codec.JSONString(mobiletypes.ConnectResultJSON{
		ShieldedPublicKey: shielded,
		EthAddress:        eth,
		RecipientInfo:     recipientInfo,
		ChainID:           chainID,
	})
}
