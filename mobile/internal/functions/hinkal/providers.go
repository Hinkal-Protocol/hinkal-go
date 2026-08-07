package hinkal

import (
	"context"
	"encoding/json"
	"time"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func ResetProviderAdapters(h *core.Hinkal) {
	h.ResetProviderAdapters()
}

func Destroy(h *core.Hinkal) error {
	return h.Destroy()
}

func DisconnectFromConnector(h *core.Hinkal) error {
	return h.DisconnectFromConnector()
}

func InitUserKeys(h *core.Hinkal, mode string) error {
	return h.InitUserKeys(context.Background(), types.LoginMessageMode(mode))
}

func StoreAndGetInitialSignature(h *core.Hinkal, authSignature string, isSolanaLedger bool, txMessageForSolanaLedger string) (string, error) {
	return h.StoreAndGetInitialSignature(
		context.Background(),
		authSignature,
		isSolanaLedger,
		txMessageForSolanaLedger,
	)
}

func SignMessage(h *core.Hinkal, message string) (string, error) {
	return h.SignMessage(context.Background(), message)
}

func SignTypedData(h *core.Hinkal, typedDataHash []byte) (string, error) {
	return h.SignTypedData(context.Background(), typedDataHash)
}

func WaitForTransaction(h *core.Hinkal, chainID int64, txHash string, confirmations, timeoutSec int64) (bool, error) {
	if confirmations < 0 {
		return false, mobileerrors.ErrNegativeConfirmations
	}
	ctx := context.Background()
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}
	return h.WaitForTransaction(ctx, int(chainID), txHash, uint64(confirmations))
}

func MonitorConnectedAddress(h *core.Hinkal, chainID int64) error {
	return h.MonitorConnectedAddress(context.Background(), int(chainID))
}

func SwitchNetwork(h *core.Hinkal, networkJSON string) error {
	var network types.EthereumNetwork
	if err := json.Unmarshal([]byte(networkJSON), &network); err != nil {
		return mobileerrors.InvalidJSON("networkJSON", err)
	}
	return h.SwitchNetwork(network)
}

func SwitchNetworkByChainID(h *core.Hinkal, chainID int64) error {
	return h.SwitchNetworkByChainID(int(chainID))
}

func IsSelectedNetworkSupported(h *core.Hinkal, chainID int64) bool {
	return h.IsSelectedNetworkSupported(int(chainID))
}

func GetEthereumAddress(h *core.Hinkal) (string, error) {
	return h.GetEthereumAddress(context.Background())
}

func GetEthereumAddressByChain(h *core.Hinkal, chainID int64) (string, error) {
	return h.GetEthereumAddressByChain(context.Background(), int(chainID))
}

func GetSolanaPublicKey(h *core.Hinkal) (string, error) {
	pk, err := h.GetSolanaPublicKey(context.Background())
	if err != nil {
		return "", err
	}
	return pk.String(), nil
}

func InitUserKeysWithEnclaveSignature(h *core.Hinkal, mode string) error {
	return h.InitUserKeysWithEnclaveSignature(context.Background(), types.LoginMessageMode(mode))
}
