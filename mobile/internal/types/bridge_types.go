package types

import "github.com/Hinkal-Protocol/hinkal-go/internal/types"

type BridgeQuoteJSON struct {
	Calldata       string `json:"calldata"`
	ExpectedAmount string `json:"expectedAmount"`
	NativeFee      string `json:"nativeFee"`
}

type BridgeRecipientJSON struct {
	RecipientAddress    string                    `json:"recipientAddress"`
	BridgeAmount        string                    `json:"bridgeAmount"`
	Quote               BridgeQuoteJSON           `json:"quote"`
	TemporarySubAccount types.TemporarySubAccount `json:"temporarySubAccount"`
}

type NearBridgeLegJSON struct {
	DestinationRecipient string                 `json:"destinationRecipient"`
	Amount               string                 `json:"amount"`
	DepositAddress       string                 `json:"depositAddress"`
	Quote                types.NearIntentsQuote `json:"quote"`
}

type NearBridgeResultJSON struct {
	DepositTxHash string              `json:"depositTxHash"`
	Legs          []NearBridgeLegJSON `json:"legs"`
}

type PrivateBridgeRecipientJSON struct {
	RecipientInfo       string `json:"recipientInfo"`
	RecipientEthAddress string `json:"recipientEthAddress"`
	ClaimableNonce      string `json:"claimableNonce"`
}
