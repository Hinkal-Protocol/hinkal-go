package types

import "math/big"

const (
	zeroAddress     = "0x0000000000000000000000000000000000000000"
	defaultFeeToken = zeroAddress
)

type FeeStructure struct {
	FeeToken     string   `json:"feeToken"`
	FlatFee      *big.Int `json:"flatFee"`
	VariableRate *big.Int `json:"variableRate"` // beeps = 0.01 of 1%
}

type FeeStructureJSON struct {
	FeeToken     string `json:"feeToken"`
	FlatFee      string `json:"flatFee"`
	VariableRate string `json:"variableRate"`
}

func ZeroFeeStructure() FeeStructure {
	return FeeStructure{
		FeeToken:     defaultFeeToken,
		FlatFee:      big.NewInt(0),
		VariableRate: big.NewInt(0),
	}
}

type ProoflessFeeStructure struct {
	FeeRecipient string   `json:"feeRecipient"`
	FeeToken     string   `json:"feeToken"`
	FeeAmount    *big.Int `json:"feeAmount"`
}

func ZeroProoflessFeeStructure() ProoflessFeeStructure {
	return ProoflessFeeStructure{
		FeeRecipient: zeroAddress,
		FeeToken:     zeroAddress,
		FeeAmount:    big.NewInt(0),
	}
}

// HinkalConfig holds configuration options for a Hinkal instance.
type HinkalConfig struct {
	GenerateProofRemotely               *bool
	DisableMerkleTreeUpdates            bool
	CacheDevice                         ICacheDevice
	CacheFilePath                       string
	UseFileCache                        bool
	DisableCaching                      bool
	SerializedCache                     map[string]string
	TronChainOverride                   int
	AllowParallelBalanceLocalDecryption bool
}

// LoginMessageMode selects which message the user signs to derive their UserKeys.
type LoginMessageMode string

const (
	LoginMessageModeProtocol        LoginMessageMode = "protocol"
	LoginMessageModePrivateTransfer LoginMessageMode = "privateTransfer"
)

const (
	SigningMessage                = "Sign to deterministically derive your Hinkal shielded account keys."
	PrivateTransferSigningMessage = "Sign to deterministically derive your Hinkal shielded account keys. \nWARNING: Only sign this on official Hinkal domains. Signing it anywhere else can expose your funds to theft."
)
