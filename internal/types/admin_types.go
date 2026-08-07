package types

type AdminTransactionType string

const (
	AdminPrivateToPrivateSend     AdminTransactionType = "private_to_private_send"
	AdminProxyToPrivateSend       AdminTransactionType = "proxy_to_private_send"
	AdminPrivateToPublicSend      AdminTransactionType = "private_to_public_send"
	AdminPublicToPublicSend       AdminTransactionType = "public_to_public_send"
	AdminPublicToPublicBridgeSend AdminTransactionType = "public_to_public_bridge_send"
	AdminPrivateSwap              AdminTransactionType = "private_swap"
	AdminPublicSwap               AdminTransactionType = "public_swap"
	AdminPublicBridgeSwap         AdminTransactionType = "public_bridge_swap"
	AdminUnshield                 AdminTransactionType = "unshield"
	AdminShield                   AdminTransactionType = "shield"
	AdminAutoShield               AdminTransactionType = "auto_shield"
	AdminPaymentLink              AdminTransactionType = "payment_link"
	AdminPublicDappTransaction    AdminTransactionType = "public_dapp_transaction"

	// PAY app action types
	AdminPayPrivateToPublicSend      AdminTransactionType = "pay_private_to_public_send"
	AdminPayPrivateToPrivateSend     AdminTransactionType = "pay_private_to_private_send"
	AdminPayPublicToPublicSend       AdminTransactionType = "pay_public_to_public_send"
	AdminPayPublicToPrivateSend      AdminTransactionType = "pay_public_to_private_send"
	AdminPayPublicToPublicBridgeSend AdminTransactionType = "pay_public_to_public_bridge_send"
)

type SwapPairToken struct {
	Symbol  string `json:"symbol"`
	Address string `json:"address"`
}

type SwapPair struct {
	TokenIn  SwapPairToken `json:"tokenIn"`
	TokenOut SwapPairToken `json:"tokenOut"`
}

type AdminDataType struct {
	HashedOwner         string               `json:"hashedOwner"`
	ChainID             int                  `json:"chainId"`
	Timestamp           int64                `json:"timestamp"`
	Action              AdminTransactionType `json:"action"`
	Erc20TokenAddresses []string             `json:"erc20TokenAddresses"`
	Erc20TokenAmount    []string             `json:"erc20TokenAmount"`
	SwapPair            *SwapPair            `json:"swapPair,omitempty"`
}
