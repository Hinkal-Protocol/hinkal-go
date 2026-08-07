package mobile

import (
	hinkal "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/hinkal"
	quotes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/quotes"
)

func (h *Hinkal) GetRecipientInfo() (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.GetRecipientInfo(h.c.h)
}

func (h *Hinkal) Deposit(chainID int64, tokenAddrsJSON, amountsWeiJSON string, preEstimateGas, returnTxData bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.Deposit(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, preEstimateGas, returnTxData)
}

func (h *Hinkal) DepositForOther(chainID int64, tokenAddrsJSON, amountsWeiJSON, recipientInfo string, preEstimateGas, returnTxData bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.DepositForOther(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, recipientInfo, preEstimateGas, returnTxData)
}

func (h *Hinkal) DepositSolana(chainID int64, tokenAddr, amountWei string, returnTxData bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.DepositSolana(h.c.h, chainID, tokenAddr, amountWei, returnTxData)
}

func (h *Hinkal) DepositSolanaForOther(chainID int64, tokenAddr, amountWei, recipientInfo string, returnTxData bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.DepositSolanaForOther(h.c.h, chainID, tokenAddr, amountWei, recipientInfo, returnTxData)
}

func (h *Hinkal) ProoflessDeposit(chainID int64, tokenAddrsJSON, amountsWeiJSON, stealthAddressStructuresJSON string, createBlockedUtxos bool, feeStructureJSON, orderID string, returnTxData bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.ProoflessDeposit(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, stealthAddressStructuresJSON, createBlockedUtxos, feeStructureJSON, orderID, returnTxData)
}

func (h *Hinkal) ProoflessDepositWithPublicFee(chainID int64, tokenAddr, amountsWeiJSON, stealthAddressStructuresJSON, feeAmountWei string, createBlockedUtxos bool, orderID string, returnTxData bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.ProoflessDepositWithPublicFee(h.c.h, chainID, tokenAddr, amountsWeiJSON, stealthAddressStructuresJSON, feeAmountWei, createBlockedUtxos, orderID, returnTxData)
}

func (h *Hinkal) Withdraw(chainID int64, tokenAddrsJSON, amountsWeiJSON, recipient string, relayerOff bool, feeToken, feeStructureJSON string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.Withdraw(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, recipient, relayerOff, feeToken, feeStructureJSON)
}

func (h *Hinkal) Transfer(chainID int64, tokenAddrsJSON, amountsWeiJSON, recipient, feeToken, feeStructureJSON string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.Transfer(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, recipient, feeToken, feeStructureJSON)
}

func (h *Hinkal) ClaimUtxo(chainID int64, tokenAddr, handle, feeStructureJSON, claimableSignature string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.ClaimUtxo(h.c.h, h.c.claimable, chainID, tokenAddr, handle, feeStructureJSON, claimableSignature)
}

func (h *Hinkal) DepositAndWithdraw(chainID int64, tokenAddr, recipientAmountsJSON, recipientAddressesJSON string, scheduleTimeSec int64, feeStructureJSON string, preEstimateGas bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.DepositAndWithdraw(h.c.h, chainID, tokenAddr, recipientAmountsJSON, recipientAddressesJSON, scheduleTimeSec, feeStructureJSON, preEstimateGas)
}

func (h *Hinkal) DepositAndBridge(chainID int64, tokenAddr, recipientsJSON string, scheduleTimeSec int64, feeStructureJSON string, preEstimateGas bool) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.DepositAndBridge(h.c.h, chainID, tokenAddr, recipientsJSON, scheduleTimeSec, feeStructureJSON, preEstimateGas)
}

func (h *Hinkal) NearDepositAndBridge(chainID int64, tokenAddr, recipientAmountsJSON, recipientAddressesJSON, paramsJSON string, scheduleTimeSec int64, feeStructureJSON string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.NearDepositAndBridge(h.c.h, chainID, tokenAddr, recipientAmountsJSON, recipientAddressesJSON, paramsJSON, scheduleTimeSec, feeStructureJSON)
}

func (h *Hinkal) CheckSendTransactionStatus(scheduleID string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.CheckSendTransactionStatus(h.c.h, scheduleID)
}

func (h *Hinkal) WithdrawStuckUtxos(chainID int64, tokenAddr, recipientAddress string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.WithdrawStuckUtxos(h.c.h, chainID, tokenAddr, recipientAddress)
}

func (h *Hinkal) Swap(chainID int64, tokenAddrsJSON, amountsWeiJSON, actionID, swapData, feeToken, feeStructureJSON string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.Swap(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, actionID, swapData, feeToken, feeStructureJSON)
}

func (h *Hinkal) SwapSolana(chainID int64, tokenAddrsJSON, amountsWeiJSON, swapData, feeToken, feeStructureJSON string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.SwapSolana(h.c.h, chainID, tokenAddrsJSON, amountsWeiJSON, swapData, feeToken, feeStructureJSON)
}

func (h *Hinkal) EmporiumOp(contract, callDataString string, invokeWallet bool, valueWei string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return hinkal.EmporiumOp(h.c.h, contract, callDataString, invokeWallet, valueWei)
}

func (h *Hinkal) GetEvmSwapPrices(chainID int64, inAmount, inTokenAddr, outTokenAddr string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return quotes.GetEvmSwapPricesJSON(chainID, inAmount, inTokenAddr, outTokenAddr)
}

func (h *Hinkal) GetSolanaSwapPrices(chainID int64, inAmount, inTokenAddr, outTokenAddr string) (string, error) {
	h.c.mu.RLock()
	defer h.c.mu.RUnlock()
	return quotes.GetSolanaSwapPricesJSON(chainID, inAmount, inTokenAddr, outTokenAddr)
}

func (h *Hinkal) BridgePrivateToPrivate(sourceChainID int64, sourceTokenAddr string, destChainID int64, destTokenAddr, amount, recipientJSON string, slippage float64, feeToken string) (string, error) {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return hinkal.BridgePrivateToPrivate(h.c.h, h.c.claimable, sourceChainID, sourceTokenAddr, destChainID, destTokenAddr, amount, recipientJSON, slippage, feeToken)
}
