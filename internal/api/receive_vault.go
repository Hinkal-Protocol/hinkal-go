package api

import (
	"context"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type checkReceiveVaultResponse struct {
	IsReceiveVault bool `json:"isReceiveVault"`
}

var (
	receiveVaultAddressChecksMu sync.Mutex
	receiveVaultAddressChecks   = map[string]bool{}
)

func CheckReceiveVaultAddress(ctx context.Context, vaultAddress string) (bool, error) {
	receiveVaultAddressChecksMu.Lock()
	cached, ok := receiveVaultAddressChecks[vaultAddress]
	receiveVaultAddressChecksMu.Unlock()
	if ok {
		return cached, nil
	}

	var resp checkReceiveVaultResponse
	body := map[string]any{"vaultAddress": vaultAddress}
	if err := Post(ctx, constants.GetEnclaveURL()+constants.ServerConfig.CheckReceiveVault, body, &resp); err != nil {
		return false, err
	}

	receiveVaultAddressChecksMu.Lock()
	receiveVaultAddressChecks[vaultAddress] = resp.IsReceiveVault
	receiveVaultAddressChecksMu.Unlock()
	return resp.IsReceiveVault, nil
}
