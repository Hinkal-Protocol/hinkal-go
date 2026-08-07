package mobile

import (
	"sync"

	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

type Client struct {
	mu        sync.RWMutex
	h         *core.Hinkal
	claimable map[string]*utxo.Utxo
}

func NewClient() *Client {
	return newClient(nil)
}

func NewClientWithConfig(configJSON string) (*Client, error) {
	if configJSON == "" {
		return NewClient(), nil
	}
	cfg, err := codec.DecodeClientConfig(configJSON)
	if err != nil {
		return nil, err
	}
	return newClient(cfg), nil
}

func newClient(cfg *types.HinkalConfig) *Client {
	return &Client{h: core.NewHinkal(cfg), claimable: map[string]*utxo.Utxo{}}
}
