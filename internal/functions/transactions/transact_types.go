package transactions

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SubmitMode string

const (
	SubmitModeRelayer   SubmitMode = "relayer"
	SubmitModeSelf      SubmitMode = "self"
	SubmitModeProofOnly SubmitMode = "proof_only"
)

type TransactSubmit struct {
	Mode    SubmitMode
	Relayer *RelayerSubmit
	Self    *SelfSubmit
}

type RelayerSubmit struct {
	AdminData             *types.AdminDataType
	AuthorizationData     *types.AuthorizationData
	WithUniswapWorkAround bool
}

type SelfSubmit struct {
	Erc20Tokens     []types.ERC20Token
	ApprovalAmounts []*big.Int
	PreEstimateGas  bool
	ReturnTxData    bool
}

func NewRelayerSubmit(submit RelayerSubmit) TransactSubmit {
	return TransactSubmit{Mode: SubmitModeRelayer, Relayer: &submit}
}

func NewSelfSubmit(submit SelfSubmit) TransactSubmit {
	return TransactSubmit{Mode: SubmitModeSelf, Self: &submit}
}

func NewProofOnlySubmit() TransactSubmit {
	return TransactSubmit{Mode: SubmitModeProofOnly}
}

type TransactProof struct {
	ZkCallData               types.NewZkCallDataType
	CircomData               types.CircomDataType
	DimData                  types.DimDataType
	CommitmentValidationData *types.CommitmentValidationDataType
	TronProofSignature       *api.TronProofSignature
}

type HinkalTransactParams struct {
	ChainID                int
	Erc20Addresses         []string
	AmountChanges          []*big.Int
	ExternalAddress        string
	ExternalActionID       types.ExternalActionID
	ExternalActionMetadata []string
	FeeStructure           *types.FeeStructure
	Relay                  string
	OnChainCreation        []bool
	RecipientAddress       string
	RecipientAmounts       []*big.Int
	SelfOutputAmounts      []*big.Int
	UseBlockedUtxos        bool
	CreateBlockedUtxos     bool
	ForceEmptyUtxos        bool
	SkipLock               bool
	MessageSeed            *big.Int
	OriginalSender         string
	Speculative            *types.SpeculativeTreeParams
	SlippageValues         []*big.Int

	Submit TransactSubmit

	InputUtxos           [][]*utxo.Utxo
	UserKeys             *cryptokeys.UserKeys
	SubAccountPrivateKey string
	EmporiumOps          []string
	OnTxConfirm          func(types.CircomDataType) error
}

type TransactResult struct {
	TxHash    string
	TxRequest types.TransactionRequest
	Proof     TransactProof
}
