package mobile

import (
	fees "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/fees"
)

func (c *Client) GetFeeStructureJSON(
	chainID64 int64,
	feeTokenAddr, tokenAddrsJSON, actionID string,
	callsJSON string,
	variableRateWei string,
	solanaParamsJSON string,
) (string, error) {
	return fees.GetFeeStructureJSON(chainID64, feeTokenAddr, tokenAddrsJSON, actionID, callsJSON, variableRateWei, solanaParamsJSON)
}

func (h *Hinkal) GetFeeStructure(
	chainID64 int64,
	feeTokenAddr, tokenAddrsJSON, actionID string,
	callsJSON string,
	variableRateWei string,
	solanaParamsJSON string,
) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return fees.GetFeeStructureJSON(chainID64, feeTokenAddr, tokenAddrsJSON, actionID, callsJSON, variableRateWei, solanaParamsJSON)
}

func (c *Client) GetGasTokenSymbols(chainID64 int64) (string, error) {
	return fees.GetGasTokenSymbols(chainID64)
}

func (c *Client) CalculateTotalFee(amountWei, feeStructureJSON string) (string, error) {
	return fees.CalculateTotalFee(amountWei, feeStructureJSON)
}

func (c *Client) CalculateWithdrawalAmount(amountWithFeeWei, feeStructureJSON string) (string, error) {
	return fees.CalculateWithdrawalAmount(amountWithFeeWei, feeStructureJSON)
}

func (c *Client) CalculateModifiedFeeStructure(chainID64 int64, tokenAddr, amountWei, feeStructureJSON string) (string, error) {
	return fees.CalculateModifiedFeeStructure(chainID64, tokenAddr, amountWei, feeStructureJSON)
}
