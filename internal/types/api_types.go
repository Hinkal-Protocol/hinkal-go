package types

import "math/big"

type CallInfo struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Calldata string   `json:"calldata"`
	Value    *big.Int `json:"value"`
}
