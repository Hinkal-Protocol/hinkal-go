package types

type ConnectResultJSON struct {
	ShieldedPublicKey string `json:"shieldedPublicKey"`
	EthAddress        string `json:"ethAddress"`
	RecipientInfo     string `json:"recipientInfo"`
	ChainID           int    `json:"chainId"`
}
