package types

import "math/big"

type SignedEnclaveResponse struct {
	Data      string `json:"data"`
	Signature string `json:"signature"`
}

type PrepareTxParams struct {
	ChainID                int
	Erc20Addresses         []string
	AmountChanges          []*big.Int
	ExternalAddress        string
	OriginalSender         string
	Relay                  string
	FeeStructure           *FeeStructure
	ExternalActionID       ExternalActionID
	ExternalActionMetadata string
	OnChainCreation        []bool
	RecipientAddress       string
	RecipientAmounts       []*big.Int
	InputCommitments       []string
	UseBlockedUtxos        bool
	ForceEmptyUtxos        bool
	SkipLock               bool
	MessageSeed            *big.Int
}

type PrepareTxRequestType struct {
	ChainID                string            `json:"chainId"`
	Erc20Addresses         []string          `json:"erc20Addresses"`
	AmountChanges          []string          `json:"amountChanges"`
	ExternalAddress        string            `json:"externalAddress"`
	OriginalSender         string            `json:"originalSender,omitempty"`
	Relay                  string            `json:"relay,omitempty"`
	FeeStructure           *FeeStructureJSON `json:"feeStructure,omitempty"`
	ExternalActionID       string            `json:"externalActionId,omitempty"`
	ExternalActionMetadata string            `json:"externalActionMetadata,omitempty"`
	OnChainCreation        []bool            `json:"onChainCreation,omitempty"`
	RecipientAddress       string            `json:"recipientAddress,omitempty"`
	RecipientAmounts       []string          `json:"recipientAmounts,omitempty"`
	InputCommitments       []string          `json:"inputCommitments,omitempty"`
	UseBlockedUtxos        bool              `json:"useBlockedUtxos,omitempty"`
	ForceEmptyUtxos        bool              `json:"forceEmptyUtxos,omitempty"`
	SkipLock               bool              `json:"skipLock,omitempty"`
	MessageSeed            string            `json:"messageSeed,omitempty"`
	NullifyingKey          string            `json:"nullifyingKey"`
	SpendingPublicKey      [2]string         `json:"spendingPublicKey"`
}

type EchoedPrepareTxRequestType struct {
	ChainID                string            `json:"chainId"`
	Erc20Addresses         []string          `json:"erc20Addresses"`
	AmountChanges          []string          `json:"amountChanges"`
	ExternalAddress        string            `json:"externalAddress"`
	OriginalSender         string            `json:"originalSender,omitempty"`
	Relay                  string            `json:"relay"`
	FeeStructure           *FeeStructureJSON `json:"feeStructure"`
	ExternalActionID       string            `json:"externalActionId"`
	ExternalActionMetadata string            `json:"externalActionMetadata"`
	OnChainCreation        []bool            `json:"onChainCreation"`
	RecipientAddress       string            `json:"recipientAddress"`
	RecipientAmounts       []string          `json:"recipientAmounts"`
	InputCommitments       []string          `json:"inputCommitments"`
	UseBlockedUtxos        bool              `json:"useBlockedUtxos"`
	ForceEmptyUtxos        bool              `json:"forceEmptyUtxos,omitempty"`
	SkipLock               bool              `json:"skipLock"`
	MessageSeed            string            `json:"messageSeed"`
}

type PreparedJobType struct {
	JobID             string `json:"jobId"`
	SignedMessageHash string `json:"signedMessageHash"`
}

type PrepareTxResponseType struct {
	PreparedJobType
	Request EchoedPrepareTxRequestType `json:"request"`
}

type PrepareStuckWithdrawRequestType struct {
	ChainID               string           `json:"chainId"`
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

type FinalizeTxRelayRequestType struct {
	JobID                 string             `json:"jobId"`
	EddsaSignature        EddsaSignatureWire `json:"eddsaSignature"`
	ChainID               string             `json:"chainId"`
	AdminData             *AdminDataType     `json:"adminData,omitempty"`
	AuthorizationData     *AuthorizationData `json:"authorizationData,omitempty"`
	WithUniswapWorkAround *bool              `json:"withUniswapWorkAround,omitempty"`
}

type FinalizeTxRelayResponseType struct {
	TxHash string `json:"txHash"`
}
