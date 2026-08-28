package hinkal

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errProviderAdapterNotInitialized     = errors.New("ProviderAdapter is not initialized")
	errNoProviderAdapterInitialized      = errors.New("no provider adapter initialized")
	errTronProviderAdapterNotInitialized = errors.New("Tron provider adapter not initialized")
	errNotSolanaProviderAdapter          = errors.New("current provider adapter is not a Solana provider adapter")
	errNotTronProviderAdapter            = errors.New("Tron provider adapter is not a TronProviderAdapter")
	errConnectedAddressNotFound          = errors.New("connected address not found")
)

func (h *Hinkal) GetProviderAdapter(chainID *int) (types.IProviderAdapter, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	finalChainID := 0
	if chainID != nil {
		finalChainID = *chainID
	}
	if finalChainID == 0 {
		switch {
		case h.ethereumProviderAdapter != nil:
			finalChainID = constants.ChainIDs.EthMainnet
		case h.solanaProviderAdapter != nil:
			finalChainID = constants.CurrentSolanaChainID
		default:
			finalChainID = h.tronChainID
		}
	}

	var adapter types.IProviderAdapter
	switch {
	case constants.IsSolanaLike(finalChainID):
		adapter = h.solanaProviderAdapter
	case constants.IsTronLike(finalChainID):
		adapter = h.tronProviderAdapter
	default:
		adapter = h.ethereumProviderAdapter
	}
	if adapter == nil {
		return nil, errProviderAdapterNotInitialized
	}
	return adapter, nil
}

func (h *Hinkal) InitProviderAdapter(ctx context.Context, adapter types.IProviderAdapter) error {
	if adapter == nil {
		return errProviderAdapterNotInitialized
	}

	defaultChainID := constants.ChainIDs.EthMainnet
	switch adapter.(type) {
	case *providers.SolanaProviderAdapter:
		defaultChainID = constants.CurrentSolanaChainID
	case *providers.TronProviderAdapter:
		defaultChainID = h.tronChainID
	}

	h.updateProviderAdapter(defaultChainID, adapter)

	chainID, err := adapter.ConnectAndPatchProvider(ctx)
	if err != nil {
		return err
	}
	return adapter.Init(&chainID)
}

func (h *Hinkal) updateProviderAdapter(chainID int, adapter types.IProviderAdapter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case constants.IsSolanaLike(chainID):
		if h.solanaProviderAdapter != nil {
			h.solanaProviderAdapter.Release()
		}
		h.solanaProviderAdapter = adapter
	case constants.IsTronLike(chainID):
		if h.tronProviderAdapter != nil {
			h.tronProviderAdapter.Release()
		}
		h.tronProviderAdapter = adapter
	default:
		if h.ethereumProviderAdapter != nil {
			h.ethereumProviderAdapter.Release()
		}
		h.ethereumProviderAdapter = adapter
	}
}

func (h *Hinkal) ResetProviderAdapters() {
	h.mu.Lock()
	adapters := []types.IProviderAdapter{
		h.ethereumProviderAdapter,
		h.solanaProviderAdapter,
		h.tronProviderAdapter,
	}
	h.ethereumProviderAdapter = nil
	h.solanaProviderAdapter = nil
	h.tronProviderAdapter = nil
	h.mu.Unlock()

	for _, adapter := range adapters {
		if adapter != nil {
			adapter.Release()
		}
	}
}

func (h *Hinkal) snapshotsClearInterval() {
	h.mu.RLock()
	commitments := make([]commitmentsSnapshot, 0, len(h.CommitmentsSnapshotServiceByChain))
	for _, c := range h.CommitmentsSnapshotServiceByChain {
		commitments = append(commitments, c)
	}
	nullifiers := make([]nullifierSnapshot, 0, len(h.NullifierSnapshotServiceByChain))
	for _, n := range h.NullifierSnapshotServiceByChain {
		nullifiers = append(nullifiers, n)
	}
	h.mu.RUnlock()

	for _, c := range commitments {
		c.IntervalClear()
	}
	for _, n := range nullifiers {
		n.IntervalClear()
	}
}

func (h *Hinkal) Destroy() error {
	h.snapshotsClearInterval()
	h.ResetProviderAdapters()
	return nil
}

func (h *Hinkal) DisconnectFromConnector() error {
	h.mu.RLock()
	adapters := []types.IProviderAdapter{
		h.ethereumProviderAdapter,
		h.solanaProviderAdapter,
		h.tronProviderAdapter,
	}
	h.mu.RUnlock()

	var errs []error
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		if err := adapter.DisconnectFromConnector(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *Hinkal) GetSigningMessage(mode types.LoginMessageMode) string {
	if mode == types.LoginMessageModePrivateTransfer {
		return types.PrivateTransferSigningMessage
	}
	return types.SigningMessage
}

func (h *Hinkal) SignHinkalMessage(ctx context.Context, mode types.LoginMessageMode) (string, error) {
	adapter, err := h.GetProviderAdapter(nil)
	if err != nil {
		return "", err
	}
	return adapter.SignMessage(ctx, h.GetSigningMessage(mode))
}

func (h *Hinkal) InitUserKeys(ctx context.Context, mode types.LoginMessageMode) error {
	signature, err := h.SignHinkalMessage(ctx, mode)
	if err != nil {
		return err
	}
	h.UserKeys = cryptokeys.NewUserKeys(signature)
	return nil
}

func (h *Hinkal) InitUserKeysWithEnclaveSignature(ctx context.Context, mode types.LoginMessageMode) error {
	signature, err := h.SignHinkalMessage(ctx, mode)
	if err != nil {
		return err
	}
	storedSignature, err := h.StoreAndGetInitialSignature(ctx, signature)
	if err != nil {
		return err
	}
	if storedSignature != "" {
		signature = storedSignature
	}
	h.InitUserKeysWithSignature(signature)
	return nil
}

func (h *Hinkal) StoreAndGetInitialSignature(
	ctx context.Context,
	authSignature string,
) (string, error) {
	ethereumAddress, err := h.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}
	if ethereumAddress == "" {
		return "", errConnectedAddressNotFound
	}
	return enclave.StoreAndGetSignatureFromEnclave(
		ctx,
		ethereumAddress,
		authSignature,
	)
}

func (h *Hinkal) SignMessage(ctx context.Context, message string) (string, error) {
	adapter, err := h.GetProviderAdapter(nil)
	if err != nil {
		return "", err
	}
	return adapter.SignMessage(ctx, message)
}

func (h *Hinkal) SignTypedData(ctx context.Context, typedDataHash []byte) (string, error) {
	adapter, err := h.GetProviderAdapter(nil)
	if err != nil {
		return "", err
	}
	return adapter.SignTypedData(ctx, typedDataHash)
}

func (h *Hinkal) WaitForTransaction(ctx context.Context, chainID int, txHash string, confirmations uint64) (bool, error) {
	adapter, err := h.GetProviderAdapter(&chainID)
	if err != nil {
		return false, err
	}
	return adapter.WaitForTransaction(ctx, chainID, txHash, confirmations)
}

func (h *Hinkal) MonitorConnectedAddress(ctx context.Context, chainID int) error {
	address, err := h.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return err
	}
	return api.Monitor(ctx, address)
}

func (h *Hinkal) SwitchNetwork(network types.EthereumNetwork) error {
	adapter, err := h.GetProviderAdapter(nil)
	if err != nil {
		return err
	}
	currentChainID := adapter.GetChainID()
	if currentChainID != nil && network.ChainID == *currentChainID {
		return nil
	}
	return adapter.SwitchNetwork(network)
}

func (h *Hinkal) SwitchNetworkByChainID(chainID int) error {
	network, ok := constants.EthereumNetworkRegistry[chainID]
	if !ok {
		return errors.New("network not supported")
	}
	return h.SwitchNetwork(network)
}

func (h *Hinkal) IsSelectedNetworkSupported(chainID int) bool {
	_, ok := constants.EthereumNetworkRegistry[chainID]
	return ok
}

func (h *Hinkal) GetEthereumAddress(ctx context.Context) (string, error) {
	h.mu.RLock()
	adapter := h.ethereumProviderAdapter
	if adapter == nil {
		adapter = h.solanaProviderAdapter
	}
	if adapter == nil {
		adapter = h.tronProviderAdapter
	}
	h.mu.RUnlock()
	if adapter == nil {
		return "", errNoProviderAdapterInitialized
	}
	return adapter.GetAddress(ctx)
}

func (h *Hinkal) GetEthereumAddressByChain(ctx context.Context, chainID int) (string, error) {
	adapter, err := h.GetProviderAdapter(&chainID)
	if err != nil {
		return "", err
	}
	return adapter.GetAddress(ctx)
}

func (h *Hinkal) GetGasPrice(ctx context.Context, chainID int) (*big.Int, error) {
	adapter, err := h.GetProviderAdapter(&chainID)
	if err != nil {
		return nil, err
	}
	return adapter.GetGasPrice(ctx, chainID)
}

func (h *Hinkal) SendTransaction(ctx context.Context, chainID int, req types.TransactionRequest) (types.TransactionResponse, error) {
	adapter, err := h.GetProviderAdapter(&chainID)
	if err != nil {
		return types.TransactionResponse{}, err
	}
	return adapter.SendTransaction(ctx, req)
}

func (h *Hinkal) GetFetchClient(chainID int) (*ethclient.Client, error) {
	adapter, err := h.GetProviderAdapter(&chainID)
	if err != nil {
		return nil, err
	}
	return adapter.GetFetchClient(chainID)
}

func (h *Hinkal) GetTransactOpts(ctx context.Context, chainID int) (*bind.TransactOpts, error) {
	adapter, err := h.GetProviderAdapter(&chainID)
	if err != nil {
		return nil, err
	}
	return adapter.GetTransactOpts(ctx)
}

func (h *Hinkal) solanaAdapter() (*providers.SolanaProviderAdapter, error) {
	h.mu.RLock()
	adapter := h.solanaProviderAdapter
	h.mu.RUnlock()
	if adapter == nil {
		return nil, errNoProviderAdapterInitialized
	}
	solanaAdapter, ok := adapter.(*providers.SolanaProviderAdapter)
	if !ok {
		return nil, errNotSolanaProviderAdapter
	}
	return solanaAdapter, nil
}

func (h *Hinkal) GetSolanaProgram(programID solana.PublicKey) (*providers.SolanaProgram, error) {
	adapter, err := h.solanaAdapter()
	if err != nil {
		return nil, err
	}
	return adapter.GetSolanaProgram(programID)
}

func (h *Hinkal) GetSolanaPublicKey(ctx context.Context) (solana.PublicKey, error) {
	adapter, err := h.solanaAdapter()
	if err != nil {
		return solana.PublicKey{}, err
	}
	return adapter.GetSolanaPublicKey(ctx)
}

func (h *Hinkal) GetSolanaConnection() (*rpc.Client, error) {
	adapter, err := h.solanaAdapter()
	if err != nil {
		return nil, err
	}
	return adapter.GetConnection(), nil
}

func (h *Hinkal) GetTronWeb() (*providers.SignableTronClient, error) {
	h.mu.RLock()
	adapter := h.tronProviderAdapter
	h.mu.RUnlock()
	if adapter == nil {
		return nil, errTronProviderAdapterNotInitialized
	}
	tronAdapter, ok := adapter.(*providers.TronProviderAdapter)
	if !ok {
		return nil, errNotTronProviderAdapter
	}
	tronWeb := tronAdapter.GetTronWeb()
	if tronWeb == nil {
		return nil, errors.New("TronWeb not available")
	}
	return tronWeb, nil
}
