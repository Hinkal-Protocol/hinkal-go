package types

import "math/big"

type NearIntentsQuoteRequest struct {
	Dry               bool   `json:"dry"`
	SwapType          string `json:"swapType"`
	SlippageTolerance int    `json:"slippageTolerance"`
	OriginAsset       string `json:"originAsset"`
	DepositType       string `json:"depositType"`
	DestinationAsset  string `json:"destinationAsset"`
	Amount            string `json:"amount"`
	Recipient         string `json:"recipient"`
	RecipientType     string `json:"recipientType"`
	RefundTo          string `json:"refundTo"`
	RefundType        string `json:"refundType"`
	Deadline          string `json:"deadline"`
}

type NearIntentsQuote struct {
	DepositAddress   string `json:"depositAddress"`
	AmountOut        string `json:"amountOut"`
	Deadline         string `json:"deadline,omitempty"`
	TimeWhenInactive string `json:"timeWhenInactive,omitempty"`
}

type NearIntentsQuoteResponse struct {
	Quote NearIntentsQuote `json:"quote"`
}

type NearIntentsToken struct {
	AssetID         string `json:"assetId"`
	Decimals        int    `json:"decimals"`
	Blockchain      string `json:"blockchain"`
	Symbol          string `json:"symbol"`
	ContractAddress string `json:"contractAddress,omitempty"`
}

type NearBridgeParams struct {
	OriginAsset        string `json:"originAsset"`
	DestinationChainID int    `json:"destinationChainId"`
	DestinationAsset   string `json:"destinationAsset"`
	SlippageBps        *int   `json:"slippageBps,omitempty"`
}

type NearBridgeLeg struct {
	DestinationRecipient string           `json:"destinationRecipient"`
	Amount               *big.Int         `json:"amount"`
	DepositAddress       string           `json:"depositAddress"`
	Quote                NearIntentsQuote `json:"quote"`
}

type NearBridgeResult struct {
	DepositTxHash string          `json:"depositTxHash"`
	Legs          []NearBridgeLeg `json:"legs"`
}
