package types

type VolatileTokenTransferResult struct {
	Success           bool   `json:"success"`
	BalanceDifference string `json:"balanceDifference,omitempty"`
	Error             string `json:"error,omitempty"`
}
