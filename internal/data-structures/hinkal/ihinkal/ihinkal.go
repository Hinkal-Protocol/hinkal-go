package ihinkal

import (
	"context"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/merkletree"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

// IHinkal is the Go counterpart of @hinkal/sdk's public IHinkal: the external SDK
// contract that consumers program against. It mirrors sdk's IHinkal next to the
// Hinkal implementation (data-structures/Hinkal/IHinkal.ts) rather than living in a
// leaf types package: like the sdk it references the concrete Utxo type, so it must
// sit above the utxo package (which depends on types) to avoid an import cycle.
// Internal accessors (cache, provider concretes, raw balances) are not part of this
// public surface; consumers that need them depend on HinkalInternal.
type IHinkal interface {
	GetProviderAdapter(chainID *int) (types.IProviderAdapter, error)
	InitProviderAdapter(ctx context.Context, adapter types.IProviderAdapter) error
	ResetProviderAdapters()
	DisconnectFromConnector() error

	InitUserKeys(ctx context.Context, mode types.LoginMessageMode) error
	InitUserKeysWithSignature(signature string)
	InitUserKeysFromSeedPhrases(seedPhrases []string) error
	StoreAndGetInitialSignature(ctx context.Context, authSignature string) (string, error)
	SignMessage(ctx context.Context, message string) (string, error)
	SignTypedData(ctx context.Context, typedDataHash []byte) (string, error)
	Destroy() error

	GetSupportedChains() []int
	IsSelectedNetworkSupported(chainID int) bool
	SwitchNetwork(network types.EthereumNetwork) error
	SwitchNetworkByChainID(chainID int) error
	ResetMerkle(ctx context.Context, chainIDs ...int) error
	ResetMerkleTreesIfNecessary(ctx context.Context, chainIDsToCheck ...int) error
	MonitorConnectedAddress(ctx context.Context, chainID int) error
	WaitForTransaction(ctx context.Context, chainID int, txHash string, confirmations uint64) (bool, error)

	GetEthereumAddress(ctx context.Context) (string, error)
	GetEthereumAddressByChain(ctx context.Context, chainID int) (string, error)
	GetSolanaPublicKey(ctx context.Context) (solana.PublicKey, error)
	GetRecipientInfo() (string, error)
	GetShieldedPublicKey() (string, error)
	GetTotalBalances(ctx context.Context, chainIDs []int, userKeys *cryptokeys.UserKeys, ethAddress string, resetCacheBefore, useBlockedUtxos bool) (map[int][]types.TokenBalance, error)
	GetTotalBalance(ctx context.Context, chainID int, userKeys *cryptokeys.UserKeys, ethAddress string, resetCacheBefore, useBlockedUtxos bool) ([]types.TokenBalance, error)
	GetStuckShieldedBalances(ctx context.Context, chainIDs []int, userKeys *cryptokeys.UserKeys, ethAddress string) (map[int][]types.TokenBalance, error)
	GetStuckShieldedBalance(ctx context.Context, chainID int, userKeys *cryptokeys.UserKeys, ethAddress string) ([]types.TokenBalance, error)

	Deposit(ctx context.Context, chainID int, erc20Addresses []string, amountChanges []*big.Int, preEstimateGas, returnTxData bool) (types.TransactionRequest, string, error)
	DepositForOther(ctx context.Context, chainID int, erc20Addresses []string, amountChanges []*big.Int, recipientInfo string, preEstimateGas, returnTxData bool) (types.TransactionRequest, string, error)
	DepositSolana(ctx context.Context, chainID int, erc20Address string, amount *big.Int, returnTxData bool) (string, error)
	DepositSolanaForOther(ctx context.Context, chainID int, erc20Address string, amount *big.Int, recipientInfo string, returnTxData bool) (string, error)
	ProoflessDeposit(ctx context.Context, chainID int, erc20Addresses []string, amountChanges []*big.Int, stealthAddressStructures []types.StealthAddressStructure, recipientEncryptionKeysOverride []string, createBlockedUtxos bool, feeStructure types.ProoflessFeeStructure, orderID string, returnTxData bool) (types.TransactionRequest, string, error)
	Transfer(ctx context.Context, chainID int, erc20Addresses []string, amountChanges []*big.Int, recipientAddress string, feeToken string, feeStructureOverride *types.FeeStructure) (string, error)
	DepositAndWithdraw(ctx context.Context, chainID int, erc20Address string, recipientAmounts []*big.Int, recipientAddresses []string, txCompletionTime *int, feeStructureOverride *types.FeeStructure, preEstimateGas bool) (types.DepositAndSendExtendedResult, error)
	DepositAndBridge(ctx context.Context, chainID int, erc20Address string, recipients []types.BridgeRecipient, txCompletionTime *int, feeStructureOverride *types.FeeStructure, preEstimateGas bool) (types.DepositAndSendExtendedResult, error)
	NearDepositAndBridge(ctx context.Context, chainID int, erc20Address string, recipientAmounts []*big.Int, recipientAddresses []string, params types.NearBridgeParams, txCompletionTime *int, feeStructureOverride *types.FeeStructure) (types.NearBridgeResult, error)
	CheckSendTransactionStatus(ctx context.Context, scheduleID string) (types.ScheduledTransactionByIDResponse, error)
	WithdrawStuckUtxos(ctx context.Context, chainID int, erc20Address string, recipientAddress string) ([]string, error)
	ClaimUtxo(ctx context.Context, chainID int, erc20Address string, claimableUtxo *utxo.Utxo, feeStructureOverride *types.FeeStructure, claimableSignature string) (string, error)
	Withdraw(ctx context.Context, chainID int, erc20Addresses []string, amountChanges []*big.Int, recipientAddress string, isRelayerOff bool, feeToken string, feeStructureOverride *types.FeeStructure) (types.TransactionRequest, string, error)
	Swap(ctx context.Context, chainID int, erc20Addresses []string, deltaAmounts []*big.Int, externalActionID types.ExternalActionID, swapData string, feeToken string, feeStructureOverride *types.FeeStructure) (string, error)
	BridgePrivateToPrivate(ctx context.Context, sourceChainID int, sourceTokenAddress string, destChainID int, destTokenAddress string, amount string, recipient bridging.PrivateBridgeRecipient, slippage float64, feeToken string) (bridging.PrivateBridgeResult, error)
	InitUserKeysWithEnclaveSignature(ctx context.Context, mode types.LoginMessageMode) error
	GetFeeStructure(ctx context.Context, chainID int, feeTokenAddress string, erc20Addresses []string, externalActionID types.ExternalActionID, calls []types.CallInfo, variableRate *big.Int, solanaTransactionParams *api.SolanaGasEstimateParams) (types.FeeStructure, error)
	EmporiumOp(contract, callDataString string, invokeWallet bool, value *big.Int) (string, error)
	GetEvmSwapPrices(ctx context.Context, chainID int, inSwapAmount, inSwapTokenAddress, outSwapTokenAddress string) (*integrations.EVMSwapPrice, error)
	GetSolanaSwapPrices(ctx context.Context, chainID int, inSwapAmount, inSwapTokenAddress, outSwapTokenAddress string) (*integrations.SolanaSwapPrice, error)
}

// HinkalInternal is the Go-only superset contract that internal packages depend on. It
// has no counterpart in @hinkal/sdk: there, internal code uses the concrete
// Hinkal class directly (structural typing + file-level imports). Go has neither,
// so internal functions take this interface instead of the concrete *Hinkal to
// avoid import cycles. It embeds the public IHinkal and adds the accessors
// and provider concretes that the public surface intentionally hides; because it
// exposes providers types it must live above the providers package.
type HinkalInternal interface {
	IHinkal

	GetUserKeys() *cryptokeys.UserKeys
	MerkleTree(chainID int) merkletree.MerkleTree
	EncryptedOutputs(chainID int) []*types.EncryptedOutputWithSign
	Nullifiers(chainID int) map[string]struct{}
	CacheDevice() types.ICacheDevice
	HinkalAddress(chainID int) string
	GenerateProofRemotely() bool
	AllowParallelBalanceLocalDecryption() bool
	BalanceFetchingMutex(chainID int) *sync.Mutex
	AreMerkleTreeUpdatesDisabled() bool
	UpdateMerkleTreeUpdates(value bool)

	GetSigningMessage(mode types.LoginMessageMode) string
	SignHinkalMessage(ctx context.Context, mode types.LoginMessageMode) (string, error)

	GetHinkalTreeRootHash(ctx context.Context, chainID int) (*big.Int, error)
	GetRandomRelay(ctx context.Context, chainID int, markAsPending bool) (string, error)
	GetGasPrice(ctx context.Context, chainID int) (*big.Int, error)
	SendTransaction(ctx context.Context, chainID int, req types.TransactionRequest) (types.TransactionResponse, error)
	GetFetchClient(chainID int) (*ethclient.Client, error)
	GetTransactOpts(ctx context.Context, chainID int) (*bind.TransactOpts, error)

	GetSolanaProgram(programID solana.PublicKey) (*providers.SolanaProgram, error)
	GetSolanaConnection() (*rpc.Client, error)
	GetTronWeb() (*providers.SignableTronClient, error)
}
