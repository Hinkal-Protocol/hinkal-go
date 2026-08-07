package hinkal

import (
	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	utxoimpl "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	onchain "github.com/Hinkal-Protocol/hinkal-go/internal/functions/onchainutxos"
	pretx "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/signers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

type Hinkal = ihinkal.IHinkal

type Config = types.HinkalConfig

func New(cfg *Config) Hinkal { return core.NewHinkal(cfg) }

type LoginMessageMode = types.LoginMessageMode

const (
	LoginMessageModeProtocol        = types.LoginMessageModeProtocol
	LoginMessageModePrivateTransfer = types.LoginMessageModePrivateTransfer
)

var ChainIDs = constants.ChainIDs

var EthereumNetworkRegistry = constants.EthereumNetworkRegistry

var (
	IsSolanaLike = constants.IsSolanaLike
	IsTronLike   = constants.IsTronLike
)

type (
	ERC20Token   = types.ERC20Token
	TokenBalance = types.TokenBalance

	FeeStructure          = types.FeeStructure
	ProoflessFeeStructure = types.ProoflessFeeStructure

	TransactionRequest  = types.TransactionRequest
	TransactionResponse = types.TransactionResponse
	EthereumNetwork     = types.EthereumNetwork
	CallInfo            = types.CallInfo

	StealthAddressStructure = types.StealthAddressStructure
	ExternalActionID        = types.ExternalActionID

	DepositAndSendExtendedResult = types.DepositAndSendExtendedResult

	BridgeRecipient     = types.BridgeRecipient
	BridgeQuote         = types.BridgeQuote
	TemporarySubAccount = types.TemporarySubAccount

	PrivateBridgeRecipient = bridging.PrivateBridgeRecipient
	PrivateBridgeResult    = bridging.PrivateBridgeResult
	PendingEnclaveUtxo     = bridging.PendingEnclaveUtxo

	NearBridgeParams = types.NearBridgeParams
	NearBridgeResult = types.NearBridgeResult
	NearBridgeLeg    = types.NearBridgeLeg
	NearIntentsQuote = types.NearIntentsQuote

	ScheduledTransactionByIDResponse = types.ScheduledTransactionByIDResponse
	ScheduledTransactionItemStatus   = types.ScheduledTransactionItemStatus
	ScheduledTransactionStatus       = types.ScheduledTransactionStatus

	ICacheDevice     = types.ICacheDevice
	IProviderAdapter = types.IProviderAdapter

	UserKeys = cryptokeys.UserKeys

	Utxo                                   = utxoimpl.Utxo
	UtxoParams                             = types.UtxoParams
	UtxoConstructorParamsWithSenderAddress = types.UtxoConstructorParamsWithSenderAddress
)

var (
	StoreUtxoInEnclave  = enclave.StoreUtxoInEnclave
	GetUtxosFromEnclave = enclave.GetUtxosFromEnclave
)

const (
	ScheduledTransactionStatusPending           = types.ScheduledTransactionStatusPending
	ScheduledTransactionStatusProcessing        = types.ScheduledTransactionStatusProcessing
	ScheduledTransactionStatusWaitingForRelayer = types.ScheduledTransactionStatusWaitingForRelayer
	ScheduledTransactionStatusSentOnChain       = types.ScheduledTransactionStatusSentOnChain
	ScheduledTransactionStatusCompleted         = types.ScheduledTransactionStatusCompleted
	ScheduledTransactionStatusFailed            = types.ScheduledTransactionStatusFailed
)

type (
	EthersProviderAdapter = providers.EthersProviderAdapter
	TronProviderAdapter   = providers.TronProviderAdapter
	SolanaProviderAdapter = providers.SolanaProviderAdapter

	PrivateKeyEVMSigner    = signers.PrivateKeyEVMSigner
	PrivateKeyTronSigner   = signers.PrivateKeyTronSigner
	PrivateKeySolanaSigner = signers.PrivateKeySolanaSigner
)

var (
	NewEthersProviderAdapter = providers.NewEthersProviderAdapter
	NewTronProviderAdapter   = providers.NewTronProviderAdapter
	NewSolanaProviderAdapter = providers.NewSolanaProviderAdapter

	NewPrivateKeyEVMSigner    = signers.NewPrivateKeyEVMSigner
	NewPrivateKeyTronSigner   = signers.NewPrivateKeyTronSigner
	NewPrivateKeySolanaSigner = signers.NewPrivateKeySolanaSigner
)

var (
	NewUserKeys = cryptokeys.NewUserKeys

	NewUtxo    = utxoimpl.NewUtxo
	CreateFrom = utxoimpl.CreateFrom

	DecodeFromReceipt           = onchain.DecodeFromReceipt
	DecodeSolanaFromTransaction = onchain.DecodeSolanaFromTransaction
)

var (
	IsValidPrivateAddress                  = pretx.IsValidPrivateAddress
	ConstructStealthAddressStructure       = pretx.ConstructStealthAddressStructure
	GetRecipientInfoFromUserKeys           = pretx.GetRecipientInfoFromUserKeys
	GetStealthAddressStructureFromUserKeys = pretx.GetStealthAddressStructureFromUserKeys
)

type SolanaGasEstimateParams = api.SolanaGasEstimateParams

var (
	CalculateTotalFee         = fees.CalculateTotalFee
	CalculateWithdrawalAmount = fees.CalculateWithdrawalAmount
	GetGasTokenSymbols        = fees.GetGasTokenSymbols
)

type (
	OKXPrice = web3.OKXPrice

	EVMSwapPrice    = integrations.EVMSwapPrice
	SolanaSwapPrice = integrations.SolanaSwapPrice
)

var GetLifiPrice = web3.GetLifiPrice

var (
	GetAmountInWei         = web3.GetAmountInWei
	GetAmountInToken       = web3.GetAmountInToken
	GetAmountWithPrecision = web3.GetAmountWithPrecision

	ResolveERC20Tokens       = web3.ResolveERC20Tokens
	ResolveERC20TokenStrict  = web3.ResolveERC20TokenStrict
	ResolveERC20TokensStrict = web3.ResolveERC20TokensStrict
)

var (
	GetNearIntentsQuote  = api.GetNearIntentsQuote
	GetNearIntentsTokens = api.GetNearIntentsTokens
)

const (
	ErrCodeInsufficientFundsToTransact = errorhandling.ErrCodeInsufficientFundsToTransact
	ErrCodeDecimalsLimit               = errorhandling.ErrCodeDecimalsLimit
	ErrCodeUtxoLimitations             = errorhandling.ErrCodeUtxoLimitations
	ErrCodeFeesOverTransactionValue    = errorhandling.ErrCodeFeesOverTransactionValue
	ErrCodeChainAddressChanged         = errorhandling.ErrCodeChainAddressChanged
)

type (
	ErrorWithAmount              = errorhandling.ErrorWithAmount
	FeeOverTransactionValueError = errorhandling.FeeOverTransactionValueError
	ErrorWithRelayerTransaction  = errorhandling.ErrorWithRelayerTransaction
)

var (
	ErrTransactionNotConfirmed  = errorhandling.ErrTransactionNotConfirmed
	ErrTransactionTimeout       = errorhandling.ErrTransactionTimeout
	ErrTransactionReplaced      = errorhandling.ErrTransactionReplaced
	ErrTransactionCanceled      = errorhandling.ErrTransactionCanceled
	ErrSigningFailed            = errorhandling.ErrSigningFailed
	ErrUserRejectedTx           = errorhandling.ErrUserRejectedTx
	ErrUserRejectedSign         = errorhandling.ErrUserRejectedSign
	ErrSimulationFailed         = errorhandling.ErrSimulationFailed
	ErrUnsupportedNetwork       = errorhandling.ErrUnsupportedNetwork
	ErrInvalidNullifier         = errorhandling.ErrInvalidNullifier
	ErrSanctionedAddress        = errorhandling.ErrSanctionedAddress
	ErrInvalidAmount            = errorhandling.ErrInvalidAmount
	ErrInvalidToken             = errorhandling.ErrInvalidToken
	ErrInvalidZKP               = errorhandling.ErrInvalidZKP
	ErrInvalidRelayerFee        = errorhandling.ErrInvalidRelayerFee
	ErrRelayerNotAvailable      = errorhandling.ErrRelayerNotAvailable
	ErrRelayerGasEstimate       = errorhandling.ErrRelayerGasEstimate
	ErrRecipientFormatIncorrect = errorhandling.ErrRecipientFormatIncorrect
	ErrRecipientAmountInvalid   = errorhandling.ErrRecipientAmountInvalid
	ErrNotEnoughTokens          = errorhandling.ErrNotEnoughTokens
	ErrTokensDifferentChains    = errorhandling.ErrTokensDifferentChains
	ErrEmptyTokenArray          = errorhandling.ErrEmptyTokenArray
	ErrNonceHigh                = errorhandling.ErrNonceHigh
	ErrLowOutputAmount          = errorhandling.ErrLowOutputAmount
	ErrSomeError                = errorhandling.ErrSomeError
)
