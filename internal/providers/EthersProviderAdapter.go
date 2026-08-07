package providers

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	goethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errNoSigner                  = errors.New("IllegalState: no signer")
	errNoChainID                 = errors.New("no Chain Id In Provider Adapter")
	errFetchClientNotInitialised = errors.New("fetch client not initialized for chain")
)

type EthersProviderAdapter struct {
	signer       types.Signer
	chainID      *int
	fetchClients map[int]*ethclient.Client
}

func NewEthersProviderAdapter() (*EthersProviderAdapter, error) {
	a := &EthersProviderAdapter{fetchClients: make(map[int]*ethclient.Client)}
	for _, chainID := range constants.EVMChainIDs {
		rpcURL, err := constants.FetchRPCURL(chainID)
		if err != nil {
			return nil, err
		}
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			return nil, fmt.Errorf("dial chain %d: %w", chainID, err)
		}
		a.fetchClients[chainID] = client
	}
	return a, nil
}

func (a *EthersProviderAdapter) InitSigner(signer types.Signer) {
	a.signer = signer
}

func (a *EthersProviderAdapter) Init(chainID *int) error {
	if chainID != nil {
		a.chainID = chainID
	}
	if a.chainID == nil {
		return errNoChainID
	}
	return nil
}

func (a *EthersProviderAdapter) DisconnectFromConnector() error {
	return nil
}

func (a *EthersProviderAdapter) ConnectToConnector() (int, error) {
	return 0, nil
}

func (a *EthersProviderAdapter) WaitForTransaction(ctx context.Context, chainID int, txHash string, confirmations uint64) (bool, error) {
	client, ok := a.fetchClients[chainID]
	if !ok {
		return false, fmt.Errorf("%w %d", errFetchClientNotInitialised, chainID)
	}
	return waitForTransaction(ctx, client, txHash, confirmations)
}

func (a *EthersProviderAdapter) SignMessage(ctx context.Context, message string) (string, error) {
	if a.signer == nil {
		return "", errNoSigner
	}
	sig, err := a.signer.SignMessage(ctx, message)
	if err != nil {
		return "", err
	}
	return hexSignature(sig)
}

func (a *EthersProviderAdapter) SignTypedData(ctx context.Context, typedDataHash []byte) (string, error) {
	if a.signer == nil {
		return "", errNoSigner
	}
	sig, err := a.signer.SignTypedData(ctx, typedDataHash)
	if err != nil {
		return "", err
	}
	return "0x" + common.Bytes2Hex(sig), nil
}

func (a *EthersProviderAdapter) SwitchNetwork(network types.EthereumNetwork) error {
	if _, ok := a.fetchClients[network.ChainID]; !ok {
		return fmt.Errorf("%w %d", errFetchClientNotInitialised, network.ChainID)
	}
	id := network.ChainID
	a.chainID = &id
	return nil
}

func (a *EthersProviderAdapter) GetAddress(ctx context.Context) (string, error) {
	if a.signer == nil {
		return "", errNoSigner
	}
	addr, err := a.signer.GetAddress(ctx)
	if err != nil {
		return "", err
	}
	if addr == (common.Address{}) {
		return "", errors.New("IllegalState")
	}
	return addr.Hex(), nil
}

func (a *EthersProviderAdapter) OnAccountChanged() error {
	return a.Init(nil)
}

func (a *EthersProviderAdapter) OnChainChanged(chainID *int) error {
	return a.Init(chainID)
}

func (a *EthersProviderAdapter) Release() {
	for _, client := range a.fetchClients {
		client.Close()
	}
	a.signer = nil
}

func GetContractWithSigner[T any](
	a types.IProviderAdapter,
	ctx context.Context,
	chainID int,
	addr common.Address,
	newFn func(common.Address, bind.ContractBackend) (T, error),
) (T, *bind.TransactOpts, error) {
	var zero T
	client, err := a.GetFetchClient(chainID)
	if err != nil {
		return zero, nil, err
	}
	contract, err := newFn(addr, client)
	if err != nil {
		return zero, nil, fmt.Errorf("instantiate contract on chain %d: %w", chainID, err)
	}
	auth, err := a.GetTransactOpts(ctx)
	if err != nil {
		return zero, nil, err
	}
	return contract, auth, nil
}

func GetContractWithFetcher[T any](
	a types.IProviderAdapter,
	chainID int,
	addr common.Address,
	newFn func(common.Address, bind.ContractBackend) (T, error),
) (T, error) {
	var zero T
	client, err := a.GetFetchClient(chainID)
	if err != nil {
		return zero, err
	}
	return newFn(addr, client)
}

func (a *EthersProviderAdapter) GetTransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	if a.signer == nil {
		return nil, errNoSigner
	}
	addr, err := a.signer.GetAddress(ctx)
	if err != nil {
		return nil, fmt.Errorf("get address for TransactOpts: %w", err)
	}
	return &bind.TransactOpts{
		From:    addr,
		Context: ctx,
		Signer: func(_ common.Address, tx *ethtypes.Transaction) (*ethtypes.Transaction, error) {
			return a.signer.SignTransaction(ctx, tx)
		},
	}, nil
}

func (a *EthersProviderAdapter) GetFetchClient(chainID int) (*ethclient.Client, error) {
	client, ok := a.fetchClients[chainID]
	if !ok {
		return nil, fmt.Errorf("%w %d", errFetchClientNotInitialised, chainID)
	}
	return client, nil
}

// for go-ethereum we have to populate fields our own because library doesn't do for us.
// This implementation is a translated copy from abstract-signer.ts - sendTransaction method.
func (a *EthersProviderAdapter) SendTransaction(ctx context.Context, req types.TransactionRequest) (types.TransactionResponse, error) {
	if a.signer == nil {
		return types.TransactionResponse{}, errNoSigner
	}
	if a.chainID == nil {
		return types.TransactionResponse{}, errNoChainID
	}
	client, ok := a.fetchClients[*a.chainID]
	if !ok {
		return types.TransactionResponse{}, fmt.Errorf("%w %d", errFetchClientNotInitialised, *a.chainID)
	}

	from, err := a.signer.GetAddress(ctx)
	if err != nil {
		return types.TransactionResponse{}, fmt.Errorf("get address: %w", err)
	}

	to := common.HexToAddress(req.To)
	value := req.Value
	if value == nil {
		value = new(big.Int)
	}

	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return types.TransactionResponse{}, fmt.Errorf("get nonce: %w", err)
	}

	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		tip = big.NewInt(1_000_000_000) // 1 gwei fallback like in ethers
	}

	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return types.TransactionResponse{}, fmt.Errorf("get latest header: %w", err)
	}

	gasFeeCap := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)

	gasLimit := req.GasLimit
	if gasLimit == 0 {
		estimated, estimateErr := client.EstimateGas(ctx, goethereum.CallMsg{
			From:      from,
			To:        &to,
			GasFeeCap: gasFeeCap,
			GasTipCap: tip,
			Value:     value,
			Data:      req.Data,
		})
		if estimateErr != nil {
			return types.TransactionResponse{}, fmt.Errorf("estimate gas: %w", estimateErr)
		}
		gasLimit = estimated
	}

	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(int64(*a.chainID)),
		Nonce:     nonce,
		GasTipCap: tip,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     value,
		Data:      req.Data,
	})

	signed, err := a.signer.SignTransaction(ctx, tx)
	if err != nil {
		return types.TransactionResponse{}, fmt.Errorf("sign transaction: %w", err)
	}

	txHash, err := a.signer.BroadcastTransaction(ctx, signed)
	if err != nil {
		return types.TransactionResponse{}, err
	}
	return types.TransactionResponse{Hash: txHash, Transaction: signed}, nil
}

func (a *EthersProviderAdapter) ConnectAndPatchProvider(ctx context.Context) (int, error) {
	if a.signer == nil {
		return 0, errNoSigner
	}
	if a.chainID == nil {
		return 0, errNoChainID
	}
	client, ok := a.fetchClients[*a.chainID]
	if !ok {
		return *a.chainID, nil
	}
	networkID, err := client.NetworkID(ctx)
	if err != nil {
		//nolint:nilerr // fall back to the configured chainID when the network lookup fails
		return *a.chainID, nil
	}
	return int(networkID.Int64()), nil
}

func (a *EthersProviderAdapter) GetChainID() *int {
	return a.chainID
}

func (a *EthersProviderAdapter) GetGasPrice(ctx context.Context, chainID int) (*big.Int, error) {
	client, ok := a.fetchClients[chainID]
	if !ok {
		return nil, fmt.Errorf("%w %d", errFetchClientNotInitialised, chainID)
	}
	price, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get gas price for chain %d: %w", chainID, err)
	}
	return price, nil
}
