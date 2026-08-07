package types

import "math/big"

type BridgeQuote struct {
	Calldata       string   `json:"calldata"`
	ExpectedAmount *big.Int `json:"expectedAmount"`
	NativeFee      *big.Int `json:"nativeFee"`
}

type TemporarySubAccount struct {
	Index      int    `json:"index"`
	EthAddress string `json:"ethAddress"`
	PrivateKey string `json:"privateKey"`
}

type BridgeRecipient struct {
	RecipientAddress    string              `json:"recipientAddress"`
	BridgeAmount        *big.Int            `json:"bridgeAmount"`
	Quote               BridgeQuote         `json:"quote"`
	TemporarySubAccount TemporarySubAccount `json:"temporarySubAccount"`
}

type AuthorizationData struct {
	V       string `json:"v"`
	R       string `json:"r"`
	S       string `json:"s"`
	Nonce   string `json:"nonce"`
	Address string `json:"address"`
	ChainID string `json:"chainId"`
}
