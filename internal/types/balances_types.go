package types

type HyperCoreSpotState struct {
	Balances []struct {
		Coin  string `json:"coin"`
		Total string `json:"total"`
	} `json:"balances,omitempty"`
}

type HyperCorePerpsState struct {
	MarginSummary *struct {
		AccountValue string `json:"accountValue"`
	} `json:"marginSummary,omitempty"`
}
