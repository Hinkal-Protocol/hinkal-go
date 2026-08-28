package types

import "math/big"

type DimDataType struct {
	TokenNumber     int `json:"tokenNumber"`
	NullifierAmount int `json:"nullifierAmount"`
	OutputAmount    int `json:"outputAmount"`
}

type HookDataType struct {
	PostHookContract string `json:"postHookContract"`
	PreHookContract  string `json:"preHookContract"`
	PreHookMetadata  string `json:"preHookMetadata"`
	PostHookMetadata string `json:"postHookMetadata"`
}

func DefaultHookData() HookDataType {
	return HookDataType{
		PreHookContract:  zeroAddress,
		PostHookContract: zeroAddress,
		PreHookMetadata:  "0x00",
		PostHookMetadata: "0x00",
	}
}

type StealthAddressStructure struct {
	H0x            *big.Int `json:"H0x"`
	H0y            *big.Int `json:"H0y"`
	H1x            *big.Int `json:"H1x"`
	H1y            *big.Int `json:"H1y"`
	StealthAddress *big.Int `json:"stealthAddress"`
}

type StealthAddressStructureJSON struct {
	H0x            string `json:"H0x"`
	H0y            string `json:"H0y"`
	H1x            string `json:"H1x"`
	H1y            string `json:"H1y"`
	StealthAddress string `json:"stealthAddress"`
}

func DefaultStealthAddressStructure() StealthAddressStructure {
	return StealthAddressStructure{
		H0x:            big.NewInt(1),
		H0y:            big.NewInt(0),
		H1x:            big.NewInt(0),
		H1y:            big.NewInt(0),
		StealthAddress: big.NewInt(0),
	}
}

type CircomExternalActionData struct {
	ExternalAddress        string   `json:"externalAddress"`
	ExternalActionID       *big.Int `json:"externalActionId"`
	ExternalActionMetadata string   `json:"externalActionMetadata"`
}

type CircomExternalActionDataJSON struct {
	ExternalAddress        string `json:"externalAddress"`
	ExternalActionID       string `json:"externalActionId"`
	ExternalActionMetadata string `json:"externalActionMetadata"`
}

type CircomDataType struct {
	RootHashHinkal          *big.Int                 `json:"rootHashHinkal"`
	RootHashHinkalIndex     *big.Int                 `json:"rootHashHinkalIndex"`
	Erc20TokenAddresses     []string                 `json:"erc20TokenAddresses"`
	AmountChanges           []*big.Int               `json:"amountChanges"`
	InputNullifiers         [][]string               `json:"inputNullifiers"`
	OutCommitments          [][]string               `json:"outCommitments"`
	EncryptedOutputs        [][]string               `json:"encryptedOutputs"`
	OnChainEncryptedOutput  string                   `json:"onChainEncryptedOutput"`
	TimeStamp               string                   `json:"timeStamp,omitempty"`
	StealthAddressStructure StealthAddressStructure  `json:"stealthAddressStructure"`
	Relay                   string                   `json:"relay"`
	ExternalActionData      CircomExternalActionData `json:"externalActionData"`
	HookData                HookDataType             `json:"hookData"`
	CalldataHash            *big.Int                 `json:"calldataHash"`
	EmporiumMessage         *big.Int                 `json:"emporiumMessage"`
	PublicSignalCount       int                      `json:"publicSignalCount"`
	OnChainCreation         []bool                   `json:"onChainCreation"`
	SlippageValues          []*big.Int               `json:"slippageValues"`
	FeeStructure            FeeStructure             `json:"feeStructure"`
	OriginalSender          string                   `json:"originalSender"`
	ExtraData               string                   `json:"extraData"`

	NewRootHash        *big.Int `json:"newRootHash,omitempty"`
	InsertedLeafIndex  *big.Int `json:"insertedLeafIndex,omitempty"`
	CreateBlockedUtxos bool     `json:"createBlockedUtxos,omitempty"`
}

type CircomDataJSONType struct {
	RootHashHinkal          *string                      `json:"rootHashHinkal,omitempty"`
	RootHashHinkalIndex     *string                      `json:"rootHashHinkalIndex,omitempty"`
	Erc20TokenAddresses     []string                     `json:"erc20TokenAddresses"`
	AmountChanges           []string                     `json:"amountChanges"`
	InputNullifiers         [][]string                   `json:"inputNullifiers"`
	OutCommitments          [][]string                   `json:"outCommitments"`
	EncryptedOutputs        [][]string                   `json:"encryptedOutputs"`
	OnChainEncryptedOutput  string                       `json:"onChainEncryptedOutput"`
	StealthAddressStructure StealthAddressStructureJSON  `json:"stealthAddressStructure"`
	TimeStamp               string                       `json:"timeStamp,omitempty"`
	Relay                   string                       `json:"relay"`
	ExternalActionData      CircomExternalActionDataJSON `json:"externalActionData"`
	HookData                HookDataType                 `json:"hookData"`
	CalldataHash            string                       `json:"calldataHash"`
	EmporiumMessage         string                       `json:"emporiumMessage"`
	PublicSignalCount       int                          `json:"publicSignalCount"`
	OnChainCreation         []bool                       `json:"onChainCreation"`
	SlippageValues          []string                     `json:"slippageValues"`
	FeeStructure            FeeStructureJSON             `json:"feeStructure"`
	OriginalSender          string                       `json:"originalSender"`
	ExtraData               string                       `json:"extraData"`

	NewRootHash        *string `json:"newRootHash,omitempty"`
	InsertedLeafIndex  *string `json:"insertedLeafIndex,omitempty"`
	CreateBlockedUtxos bool    `json:"createBlockedUtxos,omitempty"`
}

type CommitmentValidationProof struct {
	A [2]string    `json:"a"`
	B [2][2]string `json:"b"`
	C [2]string    `json:"c"`
}

type CommitmentValidationDataType struct {
	TokenAddresses []string                  `json:"tokenAddresses"`
	InAmounts      [][]string                `json:"inAmounts"`
	InTimeStamps   [][]string                `json:"inTimeStamps"`
	InCommitments  [][]string                `json:"inCommitments"`
	InNullifiers   [][]string                `json:"inNullifiers"`
	Proof          CommitmentValidationProof `json:"proof"`
}
