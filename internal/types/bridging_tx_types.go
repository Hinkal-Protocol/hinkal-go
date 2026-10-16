package types

import "math/big"

type BridgeQuote struct {
	Calldata       string   `json:"calldata"`
	ExpectedAmount *big.Int `json:"expectedAmount"`
	NativeFee      *big.Int `json:"nativeFee"`
}

type TemporarySubAccount struct {
	Index            int    `json:"index"`
	EthAddress       string `json:"ethAddress"`
	PrivateKey       string `json:"privateKey"`
	SolanaAddress    string `json:"solanaAddress,omitempty"`
	SolanaPrivateKey string `json:"solanaPrivateKey,omitempty"`
}

type BridgeRecipient struct {
	RecipientAddress        string              `json:"recipientAddress"`
	BridgeAmount            *big.Int            `json:"bridgeAmount"`
	Quote                   BridgeQuote         `json:"quote"`
	TemporarySubAccount     TemporarySubAccount `json:"temporarySubAccount"`
	DestinationChainID      *int                `json:"destinationChainId,omitempty"`
	DestinationTokenAddress string              `json:"destinationTokenAddress,omitempty"`
}

type DepositAndBridgeQuote struct {
	SourceChainID           int               `json:"sourceChainId"`
	SourceTokenAddress      string            `json:"sourceTokenAddress"`
	DestinationChainID      int               `json:"destinationChainId"`
	DestinationTokenAddress string            `json:"destinationTokenAddress"`
	ExpectedAmounts         []*big.Int        `json:"expectedAmounts"`          // destination amount per recipient, same order as the quoted recipients
	LifiRecipients          []BridgeRecipient `json:"lifiRecipients,omitempty"` // EVM sources
	NearLegs                []NearBridgeLeg   `json:"nearLegs,omitempty"`       // Solana/Tron sources
}

type AuthorizationData struct {
	V       string `json:"v"`
	R       string `json:"r"`
	S       string `json:"s"`
	Nonce   string `json:"nonce"`
	Address string `json:"address"`
	ChainID string `json:"chainId"`
}
