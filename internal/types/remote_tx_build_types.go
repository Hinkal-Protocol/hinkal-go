package types

import (
	"encoding/json"
	"math/big"
)

type SignedEnclaveResponse struct {
	Data      string `json:"data"`
	Signature string `json:"signature"`
}

type SpeculativeUtxo struct {
	Amount            string    `json:"amount"`
	Erc20TokenAddress string    `json:"erc20TokenAddress"`
	MintAddress       string    `json:"mintAddress"`
	TimeStamp         string    `json:"timeStamp"`
	StealthAddress    string    `json:"stealthAddress"`
	IsBlocked         bool      `json:"isBlocked"`
	H0                [2]string `json:"H0"`
}

// Leaves that a pending deposit will append, so a withdraw can be proved without waiting on the
// indexer. Mutually exclusive with InputCommitments on the wire.
type SpeculativeTreeParams struct {
	PendingLeaves []string            `json:"pendingLeaves"`
	InputUtxos    [][]SpeculativeUtxo `json:"inputUtxos"`
}

type PrepareTxParams struct {
	ChainID                int
	HinkalAddress          string
	Erc20Addresses         []string
	AmountChanges          []*big.Int
	ExternalAddress        string
	OriginalSender         string
	Relay                  string
	FeeStructure           *FeeStructure
	ExternalActionID       ExternalActionID
	ExternalActionMetadata string
	OnChainCreation        []bool
	RecipientAddress       []string
	RecipientAmounts       [][]*big.Int
	SelfOutputAmounts      []*big.Int
	InputCommitments       []string
	UseBlockedUtxos        bool
	CreateBlockedUtxos     bool
	ForceEmptyUtxos        bool
	SkipLock               bool
	MessageSeed            *big.Int
	Speculative            *SpeculativeTreeParams
	SlippageValues         []*big.Int
}

type PrepareTxRequestType struct {
	ChainID                string                 `json:"chainId"`
	HinkalAddress          string                 `json:"hinkalAddress"`
	Erc20Addresses         []string               `json:"erc20Addresses"`
	AmountChanges          []string               `json:"amountChanges"`
	ExternalAddress        string                 `json:"externalAddress"`
	OriginalSender         string                 `json:"originalSender,omitempty"`
	Relay                  string                 `json:"relay,omitempty"`
	FeeStructure           *FeeStructureJSON      `json:"feeStructure,omitempty"`
	ExternalActionID       string                 `json:"externalActionId,omitempty"`
	ExternalActionMetadata string                 `json:"externalActionMetadata,omitempty"`
	OnChainCreation        []bool                 `json:"onChainCreation,omitempty"`
	RecipientAddress       []string               `json:"recipientAddress,omitempty"`
	RecipientAmounts       [][]string             `json:"recipientAmounts,omitempty"`
	SelfOutputAmounts      []string               `json:"selfOutputAmounts,omitempty"`
	InputCommitments       []string               `json:"inputCommitments,omitempty"`
	UseBlockedUtxos        bool                   `json:"useBlockedUtxos,omitempty"`
	CreateBlockedUtxos     bool                   `json:"createBlockedUtxos,omitempty"`
	ForceEmptyUtxos        bool                   `json:"forceEmptyUtxos,omitempty"`
	SkipLock               bool                   `json:"skipLock,omitempty"`
	MessageSeed            string                 `json:"messageSeed,omitempty"`
	Speculative            *SpeculativeTreeParams `json:"speculative,omitempty"`
	SlippageValues         []string               `json:"slippageValues,omitempty"`
	NullifyingKey          string                 `json:"nullifyingKey"`
	SpendingPublicKey      [2]string              `json:"spendingPublicKey"`
}

type EchoedPrepareTxRequestType struct {
	ChainID                string                 `json:"chainId"`
	HinkalAddress          string                 `json:"hinkalAddress"`
	Erc20Addresses         []string               `json:"erc20Addresses"`
	AmountChanges          []string               `json:"amountChanges"`
	ExternalAddress        string                 `json:"externalAddress"`
	OriginalSender         string                 `json:"originalSender,omitempty"`
	Relay                  string                 `json:"relay"`
	FeeStructure           *FeeStructureJSON      `json:"feeStructure"`
	ExternalActionID       string                 `json:"externalActionId"`
	ExternalActionMetadata string                 `json:"externalActionMetadata"`
	OnChainCreation        []bool                 `json:"onChainCreation"`
	RecipientAddress       []string               `json:"recipientAddress"`
	RecipientAmounts       [][]string             `json:"recipientAmounts"`
	SelfOutputAmounts      []string               `json:"selfOutputAmounts"`
	InputCommitments       []string               `json:"inputCommitments"`
	UseBlockedUtxos        bool                   `json:"useBlockedUtxos"`
	CreateBlockedUtxos     bool                   `json:"createBlockedUtxos"`
	ForceEmptyUtxos        bool                   `json:"forceEmptyUtxos,omitempty"`
	SkipLock               bool                   `json:"skipLock"`
	MessageSeed            string                 `json:"messageSeed"`
	Speculative            *SpeculativeTreeParams `json:"speculative,omitempty"`
	SlippageValues         []string               `json:"slippageValues"`
}

type PreparedJobType struct {
	JobID             string `json:"jobId"`
	SignedMessageHash string `json:"signedMessageHash"`
}

type PrepareSolanaTxInstruction struct {
	AccountIndexes []int `json:"accountIndexes"`
	Data           []int `json:"data"`
	ProgramIndex   int   `json:"programIndex"`
}

type PrepareSolanaTxAccountMeta struct {
	Pubkey     string `json:"pubkey"`
	IsSigner   bool   `json:"isSigner"`
	IsWritable bool   `json:"isWritable"`
}

type PrepareSolanaTxParams struct {
	ChainID            int
	MintAddresses      []string
	AmountChanges      []*big.Int
	RelayAddress       string
	Recipient          string
	Signer             string
	FunctionName       string
	Accounts           json.RawMessage
	OnChainCreation    []bool
	RelayerFee         *big.Int
	VariableRate       *big.Int
	SwapperAccountSalt *big.Int
	HinkalInstructions []PrepareSolanaTxInstruction
	RemainingAccounts  []PrepareSolanaTxAccountMeta
	RecipientAddress   string
	RecipientAmounts   []*big.Int
	InputCommitments   []string
	UseBlockedUtxos    bool
	MessageSeed        *big.Int
	RecipientAmount    string
	DisplayRecipient   string
	Speculative        *SpeculativeTreeParams
}

type PrepareSolanaTxRequestType struct {
	ChainID            string                       `json:"chainId"`
	MintAddresses      []string                     `json:"mintAddresses"`
	AmountChanges      []string                     `json:"amountChanges"`
	RelayAddress       string                       `json:"relayAddress"`
	Recipient          string                       `json:"recipient"`
	Signer             string                       `json:"signer,omitempty"`
	FunctionName       string                       `json:"functionName"`
	Accounts           json.RawMessage              `json:"accounts"`
	OnChainCreation    []bool                       `json:"onChainCreation,omitempty"`
	RelayerFee         string                       `json:"relayerFee,omitempty"`
	VariableRate       string                       `json:"variableRate,omitempty"`
	HinkalInstructions []PrepareSolanaTxInstruction `json:"hinkalInstructions,omitempty"`
	RemainingAccounts  []PrepareSolanaTxAccountMeta `json:"remainingAccounts,omitempty"`
	SwapperAccountSalt string                       `json:"swapperAccountSalt,omitempty"`
	RecipientAddress   string                       `json:"recipientAddress,omitempty"`
	RecipientAmounts   []string                     `json:"recipientAmounts,omitempty"`
	InputCommitments   []string                     `json:"inputCommitments,omitempty"`
	UseBlockedUtxos    bool                         `json:"useBlockedUtxos,omitempty"`
	MessageSeed        string                       `json:"messageSeed,omitempty"`
	RecipientAmount    string                       `json:"recipientAmount,omitempty"`
	DisplayRecipient   string                       `json:"displayRecipient,omitempty"`
	Speculative        *SpeculativeTreeParams       `json:"speculative,omitempty"`
	NullifyingKey      string                       `json:"nullifyingKey"`
	SpendingPublicKey  [2]string                    `json:"spendingPublicKey"`
}

type EchoedPrepareSolanaTxRequestType struct {
	ChainID            string                       `json:"chainId"`
	MintAddresses      []string                     `json:"mintAddresses"`
	AmountChanges      []string                     `json:"amountChanges"`
	RelayAddress       string                       `json:"relayAddress"`
	Recipient          string                       `json:"recipient"`
	Signer             string                       `json:"signer"`
	FunctionName       string                       `json:"functionName"`
	Accounts           json.RawMessage              `json:"accounts,omitempty"`
	OnChainCreation    []bool                       `json:"onChainCreation"`
	RelayerFee         string                       `json:"relayerFee"`
	VariableRate       string                       `json:"variableRate"`
	HinkalInstructions []PrepareSolanaTxInstruction `json:"hinkalInstructions"`
	RemainingAccounts  []PrepareSolanaTxAccountMeta `json:"remainingAccounts"`
	SwapperAccountSalt string                       `json:"swapperAccountSalt"`
	RecipientAddress   string                       `json:"recipientAddress"`
	RecipientAmounts   []string                     `json:"recipientAmounts"`
	InputCommitments   []string                     `json:"inputCommitments"`
	UseBlockedUtxos    bool                         `json:"useBlockedUtxos"`
	MessageSeed        string                       `json:"messageSeed"`
	RecipientAmount    string                       `json:"recipientAmount,omitempty"`
	DisplayRecipient   string                       `json:"displayRecipient,omitempty"`
	Speculative        *SpeculativeTreeParams       `json:"speculative,omitempty"`
}

type PrepareSolanaTxResponseType struct {
	PreparedJobType
	Request EchoedPrepareSolanaTxRequestType `json:"request"`
}

type PrepareTxResponseType struct {
	PreparedJobType
	Request EchoedPrepareTxRequestType `json:"request"`
}

type PrepareSolanaStuckWithdrawParams struct {
	ChainID               int
	MintAddress           string
	Recipient             string
	RelayAddress          string
	Accounts              json.RawMessage
	FeeStructures         map[string]FeeStructure
	HashedEthereumAddress string
}

type PrepareSolanaStuckWithdrawRequestType struct {
	ChainID               string                      `json:"chainId"`
	MintAddress           string                      `json:"mintAddress"`
	Recipient             string                      `json:"recipient"`
	RelayAddress          string                      `json:"relayAddress"`
	Accounts              json.RawMessage             `json:"accounts"`
	FeeStructures         map[string]FeeStructureJSON `json:"feeStructures"`
	HashedEthereumAddress string                      `json:"hashedEthereumAddress"`
	NullifyingKey         string                      `json:"nullifyingKey"`
	SpendingPublicKey     [2]string                   `json:"spendingPublicKey"`
}

type EchoedPrepareSolanaStuckWithdrawRequestType struct {
	ChainID               string                      `json:"chainId"`
	MintAddress           string                      `json:"mintAddress"`
	Recipient             string                      `json:"recipient"`
	RelayAddress          string                      `json:"relayAddress"`
	Signer                string                      `json:"signer"`
	Accounts              json.RawMessage             `json:"accounts,omitempty"`
	FeeStructures         map[string]FeeStructureJSON `json:"feeStructures"`
	HashedEthereumAddress string                      `json:"hashedEthereumAddress"`
}

type PrepareSolanaStuckWithdrawResponseType struct {
	Jobs    []PreparedJobType                           `json:"jobs"`
	Request EchoedPrepareSolanaStuckWithdrawRequestType `json:"request"`
}

type PrepareStuckWithdrawRequestType struct {
	ChainID               string           `json:"chainId"`
	HinkalAddress         string           `json:"hinkalAddress"`
	Erc20Address          string           `json:"erc20Address"`
	ExternalAddress       string           `json:"externalAddress"`
	Relay                 string           `json:"relay"`
	FeeStructure          FeeStructureJSON `json:"feeStructure"`
	HashedEthereumAddress string           `json:"hashedEthereumAddress"`
	NullifyingKey         string           `json:"nullifyingKey"`
	SpendingPublicKey     [2]string        `json:"spendingPublicKey"`
}

type EchoedPrepareStuckWithdrawRequestType struct {
	ChainID               string           `json:"chainId"`
	HinkalAddress         string           `json:"hinkalAddress"`
	Erc20Address          string           `json:"erc20Address"`
	ExternalAddress       string           `json:"externalAddress"`
	Relay                 string           `json:"relay"`
	FeeStructure          FeeStructureJSON `json:"feeStructure"`
	HashedEthereumAddress string           `json:"hashedEthereumAddress"`
}

type PrepareStuckWithdrawResponseType struct {
	Jobs    []PreparedJobType                     `json:"jobs"`
	Request EchoedPrepareStuckWithdrawRequestType `json:"request"`
}

type EddsaSignatureWire struct {
	R8 [2]string `json:"R8"`
	S  string    `json:"S"`
}

type FinalizeTxRequestType struct {
	JobID          string             `json:"jobId"`
	EddsaSignature EddsaSignatureWire `json:"eddsaSignature"`
}

type TronProofSignatureJSON struct {
	V int    `json:"v"`
	R string `json:"r"`
	S string `json:"s"`
}

type FinalizeTxResponseType struct {
	ZkCallData               NewZkCallDataType             `json:"zkCallData"`
	CircomData               CircomDataJSONType            `json:"circomData"`
	DimData                  DimDataType                   `json:"dimData"`
	CommitmentValidationData *CommitmentValidationDataType `json:"commitmentValidationData,omitempty"`
	TronProofSignature       *TronProofSignatureJSON       `json:"tronProofSignature,omitempty"`
}

type FinalizeSolanaTxResponseType struct {
	SolanaArgs               json.RawMessage               `json:"solanaArgs"`
	CommitmentValidationData *CommitmentValidationDataType `json:"commitmentValidationData,omitempty"`
}

type FinalizeTxRelayRequestType struct {
	JobID                 string             `json:"jobId"`
	EddsaSignature        EddsaSignatureWire `json:"eddsaSignature"`
	ChainID               string             `json:"chainId"`
	AdminData             *AdminDataType     `json:"adminData,omitempty"`
	AuthorizationData     *AuthorizationData `json:"authorizationData,omitempty"`
	WithUniswapWorkAround *bool              `json:"withUniswapWorkAround,omitempty"`
}

type FinalizeSolanaTxRelayRequestType struct {
	JobID          string             `json:"jobId"`
	EddsaSignature EddsaSignatureWire `json:"eddsaSignature"`
	ChainID        string             `json:"chainId"`
	AdminData      *AdminDataType     `json:"adminData,omitempty"`
}

type FinalizeTxRelayResponseType struct {
	TxHash string `json:"txHash"`
}
