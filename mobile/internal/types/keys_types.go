package types

type EddsaSignatureJSON struct {
	R8 []string `json:"r8"`
	S  string   `json:"s"`
}

type SpendingKeyPairJSON struct {
	PrivSpendingKey     string   `json:"privSpendingKey"`
	PubSpendingBJJPoint []string `json:"pubSpendingBJJPoint"`
}
