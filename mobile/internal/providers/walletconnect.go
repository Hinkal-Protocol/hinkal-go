package providers

import (
	"context"
	"encoding/hex"
	"math/big"
	"sync"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type HostWallet interface {
	Address() (string, error)
	ChainID() int64
	PersonalSign(message string) (string, error)
	SendTransaction(toHex string, dataHex string, valueDec string, gasLimit int64) (string, error)
	SwitchChain(chainID int64) error
}

type WalletConnectProviderAdapter struct {
	host    HostWallet
	chainID *int
	mu      sync.Mutex
	clients map[int]*ethclient.Client
}

var _ types.IProviderAdapter = (*WalletConnectProviderAdapter)(nil)

func NewWalletConnect(host HostWallet) *WalletConnectProviderAdapter {
	cid := int(host.ChainID())
	return &WalletConnectProviderAdapter{
		host:    host,
		chainID: &cid,
		clients: map[int]*ethclient.Client{},
	}
}

func (a *WalletConnectProviderAdapter) setChainID(cid int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chainID = &cid
}

func (a *WalletConnectProviderAdapter) Init(chainID *int) error {
	if chainID != nil {
		a.setChainID(*chainID)
	}
	if a.GetChainID() == nil {
		return mobileerrors.ErrNoChainID
	}
	return nil
}

func (a *WalletConnectProviderAdapter) ConnectToConnector() (int, error) {
	return int(a.host.ChainID()), nil
}

func (a *WalletConnectProviderAdapter) DisconnectFromConnector() error { return nil }

func (a *WalletConnectProviderAdapter) ConnectAndPatchProvider(_ context.Context) (int, error) {
	cid := int(a.host.ChainID())
	a.setChainID(cid)
	return cid, nil
}

func (a *WalletConnectProviderAdapter) GetChainID() *int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.chainID
}

func (a *WalletConnectProviderAdapter) SignMessage(_ context.Context, message string) (string, error) {
	return a.host.PersonalSign(message)
}

func (a *WalletConnectProviderAdapter) SignTypedData(_ context.Context, _ []byte) (string, error) {
	return "", mobileerrors.ErrSignTypedDataNotSupported
}

func (a *WalletConnectProviderAdapter) GetAddress(_ context.Context) (string, error) {
	return a.host.Address()
}

func (a *WalletConnectProviderAdapter) SwitchNetwork(network types.EthereumNetwork) error {
	if err := a.host.SwitchChain(int64(network.ChainID)); err != nil {
		return err
	}
	a.setChainID(network.ChainID)
	return nil
}

func (a *WalletConnectProviderAdapter) OnAccountChanged() error { return nil }

func (a *WalletConnectProviderAdapter) OnChainChanged(chainID *int) error {
	if chainID != nil {
		a.setChainID(*chainID)
	}
	return nil
}

func (a *WalletConnectProviderAdapter) Release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.clients {
		c.Close()
	}
	a.clients = map[int]*ethclient.Client{}
}

func (a *WalletConnectProviderAdapter) SendTransaction(
	_ context.Context, req types.TransactionRequest,
) (types.TransactionResponse, error) {
	dataHex := ""
	if len(req.Data) > 0 {
		dataHex = "0x" + hex.EncodeToString(req.Data)
	}
	valueDec := "0"
	if req.Value != nil {
		valueDec = req.Value.String()
	}
	hash, err := a.host.SendTransaction(req.To, dataHex, valueDec, int64(req.GasLimit))
	if err != nil {
		return types.TransactionResponse{}, err
	}
	return types.TransactionResponse{Hash: hash}, nil
}

func (a *WalletConnectProviderAdapter) fetchClient(chainID int) (*ethclient.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.clients[chainID]; ok {
		return c, nil
	}
	url, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return nil, err
	}
	c, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}
	a.clients[chainID] = c
	return c, nil
}

func (a *WalletConnectProviderAdapter) GetFetchClient(chainID int) (*ethclient.Client, error) {
	return a.fetchClient(chainID)
}

func (a *WalletConnectProviderAdapter) WaitForTransaction(
	ctx context.Context, chainID int, txHash string, confirmations uint64,
) (bool, error) {
	client, err := a.fetchClient(chainID)
	if err != nil {
		return false, err
	}
	return providers.WaitForEVMTransaction(ctx, client, txHash, confirmations)
}

func (a *WalletConnectProviderAdapter) GetGasPrice(ctx context.Context, chainID int) (*big.Int, error) {
	client, err := a.fetchClient(chainID)
	if err != nil {
		return nil, err
	}
	return client.SuggestGasPrice(ctx)
}

func (a *WalletConnectProviderAdapter) GetTransactOpts(_ context.Context) (*bind.TransactOpts, error) {
	return nil, mobileerrors.ErrGetTransactOptsNotSupported
}
