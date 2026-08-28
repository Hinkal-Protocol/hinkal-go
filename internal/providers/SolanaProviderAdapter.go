package providers

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errSolanaNoWallet = errors.New("no wallet provided")
var errSolanaNoChainID = errors.New("no Chain Id In Provider Adapter")

type SolanaProviderAdapter struct {
	chainID         *int
	client          *rpc.Client
	wallet          types.SolanaSigner
	ethereumAddress string
}

func NewSolanaProviderAdapter(chainID int, ethereumAddress string) (*SolanaProviderAdapter, error) {
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, err
	}
	id := chainID
	rpcClient := jsonrpc.NewClientWithOpts(rpcURL, &jsonrpc.RPCClientOpts{HTTPClient: api.SolanaFallbackHTTPClient()})
	return &SolanaProviderAdapter{
		chainID:         &id,
		client:          rpc.NewWithCustomRPCClient(rpcClient),
		ethereumAddress: ethereumAddress,
	}, nil
}

func (a *SolanaProviderAdapter) InitConnector(wallet types.SolanaSigner) {
	a.wallet = wallet
}

func (a *SolanaProviderAdapter) Init(chainID *int) error {
	if chainID != nil {
		a.chainID = chainID
	}
	return nil
}

func (a *SolanaProviderAdapter) ConnectToConnector() (int, error) {
	if a.chainID == nil {
		return 0, nil
	}
	return *a.chainID, nil
}

func (a *SolanaProviderAdapter) DisconnectFromConnector() error {
	return nil
}

func (a *SolanaProviderAdapter) ConnectAndPatchProvider(_ context.Context) (int, error) {
	if a.chainID == nil {
		return 0, errSolanaNoChainID
	}
	return *a.chainID, nil
}

func (a *SolanaProviderAdapter) GetChainID() *int {
	return a.chainID
}

func (a *SolanaProviderAdapter) WaitForTransaction(ctx context.Context, _ int, txHash string, _ uint64) (bool, error) {
	return waitForSolanaTransaction(ctx, a.client, txHash)
}

func (a *SolanaProviderAdapter) SignMessage(ctx context.Context, message string) (string, error) {
	if a.wallet == nil {
		return "", errorhandling.ErrSigningFailed
	}
	sig, err := a.wallet.SignMessage(ctx, []byte(message))
	if err != nil {
		return "", err
	}
	return hexSignature(sig)
}

func (a *SolanaProviderAdapter) GetAddress(ctx context.Context) (string, error) {
	if a.ethereumAddress != "" {
		return a.ethereumAddress, nil
	}
	if a.wallet == nil {
		return "", errors.New("IllegalState")
	}
	pubKey, err := a.wallet.GetPublicKey(ctx)
	if err != nil {
		return "", err
	}
	return pubKey.String(), nil
}

func (a *SolanaProviderAdapter) SwitchNetwork(network types.EthereumNetwork) error {
	id := network.ChainID
	a.chainID = &id
	return nil
}

func (a *SolanaProviderAdapter) SignTypedData(_ context.Context, _ []byte) (string, error) {
	return "", errors.New("typed data signing not supported on Solana")
}

func (a *SolanaProviderAdapter) OnAccountChanged() error {
	return nil
}

func (a *SolanaProviderAdapter) OnChainChanged(chainID *int) error {
	return a.Init(chainID)
}

func (a *SolanaProviderAdapter) Release() {
	a.wallet = nil
}

func (a *SolanaProviderAdapter) GetTransactOpts(_ context.Context) (*bind.TransactOpts, error) {
	return nil, errors.New("not implemented from SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) GetFetchClient(_ int) (*ethclient.Client, error) {
	return nil, errors.New("not implemented from SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) SendTransaction(_ context.Context, _ types.TransactionRequest) (types.TransactionResponse, error) {
	return types.TransactionResponse{}, errors.New("not implemented from SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) GetGasPrice(_ context.Context, _ int) (*big.Int, error) {
	return nil, errors.New("not implemented from SolanaProviderAdapter")
}

// Solana-specific methods

func (a *SolanaProviderAdapter) GetConnection() *rpc.Client {
	return a.client
}

func (a *SolanaProviderAdapter) GetSolanaPublicKey(ctx context.Context) (solana.PublicKey, error) {
	if a.wallet == nil {
		return solana.PublicKey{}, errSolanaNoWallet
	}
	return a.wallet.GetPublicKey(ctx)
}

func (a *SolanaProviderAdapter) GetSolanaProgram(programID solana.PublicKey) (*SolanaProgram, error) {
	if a.wallet == nil {
		return nil, errSolanaNoWallet
	}
	return &SolanaProgram{
		ProgramID: programID,
		Client:    a.client,
		Signer:    a.wallet,
	}, nil
}

func (a *SolanaProviderAdapter) SignTransactionWithoutBroadcast(ctx context.Context, tx *solana.Transaction) (string, string, error) {
	program, err := a.GetSolanaProgram(solana.PublicKey{})
	if err != nil {
		return "", "", err
	}
	return program.SignOnly(ctx, tx)
}
