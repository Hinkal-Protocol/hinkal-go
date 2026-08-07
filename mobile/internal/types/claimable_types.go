package types

type ClaimableUtxoJSON struct {
	Handle        string `json:"handle"`
	Amount        string `json:"amount"`
	TokenAddress  string `json:"tokenAddress"`
	SenderAddress string `json:"senderAddress"`
	Timestamp     string `json:"timestamp"`
}

type PendingEnclaveUtxoJSON struct {
	ChainID            int               `json:"chainId"`
	SenderAddress      string            `json:"senderAddress"`
	RecipientAddress   string            `json:"recipientAddress"`
	ClaimableSignature string            `json:"claimableSignature"`
	Utxo               ClaimableUtxoJSON `json:"utxo"`
}

type PrivateBridgeResultJSON struct {
	SourceTxHash       string                  `json:"sourceTxHash"`
	DestTxHash         string                  `json:"destTxHash"`
	PendingEnclaveUtxo *PendingEnclaveUtxoJSON `json:"pendingEnclaveUtxo,omitempty"`
}
