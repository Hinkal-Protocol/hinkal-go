package providers

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errTronNoChainID          = errors.New("TronProviderAdapter: Chain Id Not Set")
	errTronNoChainIDInAdapter = errors.New("no Chain Id In Provider Adapter")
)

type TronProviderAdapter struct {
	chainID   *int
	evmClient *ethclient.Client
	tronWeb   *SignableTronClient
}

func NewTronProviderAdapter(chainID int) *TronProviderAdapter {
	id := chainID
	return &TronProviderAdapter{chainID: &id}
}

func (a *TronProviderAdapter) InitConnector(ctx context.Context, connector types.TronSigner) error {
	if a.chainID == nil {
		return errTronNoChainID
	}
	address, err := connector.GetAddress(ctx)
	if err != nil {
		return fmt.Errorf("get address: %w", err)
	}
	walletClient, err := tron.NewWalletClient(*a.chainID)
	if err != nil {
		return err
	}
	evmRPCURL, err := constants.FetchRPCURL(*a.chainID)
	if err != nil {
		return err
	}
	evmClient, err := ethclient.Dial(evmRPCURL)
	if err != nil {
		return fmt.Errorf("dial tron EVM compat RPC: %w", err)
	}
	a.evmClient = evmClient
	a.tronWeb = newSignableTronClient(walletClient, connector, address)
	return nil
}

func (a *TronProviderAdapter) Init(chainID *int) error {
	a.chainID = chainID
	return nil
}

func (a *TronProviderAdapter) ConnectToConnector() (int, error) {
	if a.chainID == nil {
		return 0, nil
	}
	return *a.chainID, nil
}

func (a *TronProviderAdapter) DisconnectFromConnector() error {
	return nil
}

func (a *TronProviderAdapter) ConnectAndPatchProvider(_ context.Context) (int, error) {
	if a.chainID == nil {
		return 0, errTronNoChainIDInAdapter
	}
	return *a.chainID, nil
}

func (a *TronProviderAdapter) GetChainID() *int {
	return a.chainID
}

func (a *TronProviderAdapter) WaitForTransaction(ctx context.Context, _ int, txHash string, confirmations uint64) (bool, error) {
	if a.tronWeb == nil {
		return false, errors.New("IllegalState: tronWeb not initialized, call InitConnector first")
	}
	return tron.WaitForTransaction(ctx, a.tronWeb.WalletClient(), txHash, confirmations)
}

func (a *TronProviderAdapter) SignMessage(ctx context.Context, message string) (string, error) {
	if a.tronWeb == nil {
		return "", errors.New("IllegalState: no signer, call InitConnector first")
	}
	sig, err := a.tronWeb.signer.SignMessage(ctx, message)
	if err != nil {
		return "", err
	}
	// Keep the provider output 0x-prefixed; Tron Ledger returns signatures without this prefix in TS.
	return hexSignature(sig)
}

func (a *TronProviderAdapter) GetAddress(_ context.Context) (string, error) {
	if a.tronWeb == nil {
		return "", errors.New("IllegalState")
	}
	return a.tronWeb.GetAddress(), nil
}

func (a *TronProviderAdapter) SwitchNetwork(network types.EthereumNetwork) error {
	id := network.ChainID
	a.chainID = &id
	return nil
}

func (a *TronProviderAdapter) SignTypedData(_ context.Context, _ []byte) (string, error) {
	return "", errors.New("typed data signing not supported on Tron")
}

func (a *TronProviderAdapter) OnAccountChanged() error {
	return nil
}

func (a *TronProviderAdapter) OnChainChanged(_ *int) error {
	return nil
}

func (a *TronProviderAdapter) Release() {
	if a.evmClient != nil {
		a.evmClient.Close()
	}
	a.tronWeb = nil
}

func (a *TronProviderAdapter) GetTransactOpts(_ context.Context) (*bind.TransactOpts, error) {
	return nil, errors.New("not implemented from TronProviderAdapter")
}

func (a *TronProviderAdapter) GetFetchClient(_ int) (*ethclient.Client, error) {
	if a.evmClient == nil {
		return nil, errors.New("IllegalState: evmClient not initialized, call InitConnector first")
	}
	return a.evmClient, nil
}

func (a *TronProviderAdapter) SendTransaction(_ context.Context, _ types.TransactionRequest) (types.TransactionResponse, error) {
	return types.TransactionResponse{}, errors.New("not implemented from TronProviderAdapter")
}

func (a *TronProviderAdapter) GetGasPrice(_ context.Context, _ int) (*big.Int, error) {
	return nil, errors.New("not implemented from TronProviderAdapter")
}

func (a *TronProviderAdapter) GetTronWeb() *SignableTronClient {
	return a.tronWeb
}
