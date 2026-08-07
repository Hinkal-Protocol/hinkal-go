package hinkal

import (
	"context"
	"encoding/json"
	"math/big"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func GetRecipientInfo(h *core.Hinkal) (string, error) {
	return h.GetRecipientInfo()
}

func Deposit(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON string, preEstimateGas, returnTxData bool) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	req, hash, err := h.Deposit(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		preEstimateGas,
		returnTxData,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTxResult(req, hash)
}

func DepositForOther(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON, recipientInfo string, preEstimateGas, returnTxData bool) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	req, hash, err := h.DepositForOther(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		recipientInfo,
		preEstimateGas,
		returnTxData,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTxResult(req, hash)
}

func DepositSolana(h *core.Hinkal, chainID int64, tokenAddr, amountWei string, returnTxData bool) (string, error) {
	amount, err := codec.DecodeBig(amountWei)
	if err != nil {
		return "", err
	}
	return h.DepositSolana(context.Background(), int(chainID), tokenAddr, amount, returnTxData)
}

func DepositSolanaForOther(h *core.Hinkal, chainID int64, tokenAddr, amountWei, recipientInfo string, returnTxData bool) (string, error) {
	amount, err := codec.DecodeBig(amountWei)
	if err != nil {
		return "", err
	}
	return h.DepositSolanaForOther(
		context.Background(),
		int(chainID),
		tokenAddr,
		amount,
		recipientInfo,
		returnTxData,
	)
}

func ProoflessDeposit(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON, stealthAddressStructuresJSON string, createBlockedUtxos bool, feeStructureJSON, orderID string, returnTxData bool) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	structures, err := codec.DecodeStealthAddressStructures(stealthAddressStructuresJSON)
	if err != nil {
		return "", err
	}
	feeStructure, err := codec.DecodeProoflessFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	req, hash, err := h.ProoflessDeposit(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		structures,
		nil,
		createBlockedUtxos,
		feeStructure,
		orderID,
		returnTxData,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTxResult(req, hash)
}

func ProoflessDepositWithPublicFee(h *core.Hinkal, chainID int64, tokenAddr, amountsWeiJSON, stealthAddressStructuresJSON, feeAmountWei string, createBlockedUtxos bool, orderID string, returnTxData bool) (string, error) {
	amounts, err := codec.DecodeBigs(amountsWeiJSON)
	if err != nil {
		return "", err
	}
	if len(amounts) == 0 {
		return "", mobileerrors.ErrEmptyAmounts
	}
	structures, err := codec.DecodeStealthAddressStructures(stealthAddressStructuresJSON)
	if err != nil {
		return "", err
	}
	feeAmount, err := codec.DecodeBig(feeAmountWei)
	if err != nil {
		return "", err
	}
	if constants.IsSolanaLike(int(chainID)) {
		return "", mobileerrors.ErrProoflessPublicFeeOnSolana
	}
	tokenAddrs := make([]string, len(amounts))
	for i := range tokenAddrs {
		tokenAddrs[i] = tokenAddr
	}
	req, hash, err := h.ProoflessDeposit(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		structures,
		nil,
		createBlockedUtxos,
		types.ProoflessFeeStructure{FeeToken: tokenAddr, FeeAmount: feeAmount},
		orderID,
		returnTxData,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTxResult(req, hash)
}

func Withdraw(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON, recipient string, relayerOff bool, feeToken, feeStructureJSON string) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	req, hash, err := h.Withdraw(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		recipient,
		relayerOff,
		feeToken,
		feeOverride,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTxResult(req, hash)
}

func Transfer(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON, recipient, feeToken, feeStructureJSON string) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	return h.Transfer(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		recipient,
		feeToken,
		feeOverride,
	)
}

func ClaimUtxo(h *core.Hinkal, claimable map[string]*utxo.Utxo, chainID int64, tokenAddr, handle, feeStructureJSON, claimableSignature string) (string, error) {
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	claimableUtxo, err := claimableByHandle(claimable, handle)
	if err != nil {
		return "", err
	}
	return h.ClaimUtxo(
		context.Background(),
		int(chainID),
		tokenAddr,
		claimableUtxo,
		feeOverride,
		claimableSignature,
	)
}

func DepositAndWithdraw(h *core.Hinkal, chainID int64, tokenAddr, recipientAmountsJSON, recipientAddressesJSON string, scheduleTimeSec int64, feeStructureJSON string, preEstimateGas bool) (string, error) {
	amounts, recipients, err := codec.DecodePairedAmountRecipients(recipientAmountsJSON, recipientAddressesJSON)
	if err != nil {
		return "", err
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	res, err := h.DepositAndWithdraw(
		context.Background(),
		int(chainID),
		tokenAddr,
		amounts,
		recipients,
		codec.OptionalSeconds(scheduleTimeSec),
		feeOverride,
		preEstimateGas,
	)
	if err != nil {
		return "", err
	}
	return codec.JSONString(res)
}

func DepositAndBridge(h *core.Hinkal, chainID int64, tokenAddr, recipientsJSON string, scheduleTimeSec int64, feeStructureJSON string, preEstimateGas bool) (string, error) {
	recipients, err := codec.DecodeBridgeRecipients(recipientsJSON)
	if err != nil {
		return "", err
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	res, err := h.DepositAndBridge(
		context.Background(),
		int(chainID),
		tokenAddr,
		recipients,
		codec.OptionalSeconds(scheduleTimeSec),
		feeOverride,
		preEstimateGas,
	)
	if err != nil {
		return "", err
	}
	return codec.JSONString(res)
}

func NearDepositAndBridge(h *core.Hinkal, chainID int64, tokenAddr, recipientAmountsJSON, recipientAddressesJSON, paramsJSON string, scheduleTimeSec int64, feeStructureJSON string) (string, error) {
	amounts, recipients, err := codec.DecodePairedAmountRecipients(recipientAmountsJSON, recipientAddressesJSON)
	if err != nil {
		return "", err
	}
	var params types.NearBridgeParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", mobileerrors.InvalidJSON("paramsJSON", err)
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	res, err := h.NearDepositAndBridge(
		context.Background(),
		int(chainID),
		tokenAddr,
		amounts,
		recipients,
		params,
		codec.OptionalSeconds(scheduleTimeSec),
		feeOverride,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeNearBridgeResult(res)
}

func CheckSendTransactionStatus(h *core.Hinkal, scheduleID string) (string, error) {
	res, err := h.CheckSendTransactionStatus(context.Background(), scheduleID)
	if err != nil {
		return "", err
	}
	return codec.JSONString(res)
}

func WithdrawStuckUtxos(h *core.Hinkal, chainID int64, tokenAddr, recipientAddress string) (string, error) {
	hashes, err := h.WithdrawStuckUtxos(
		context.Background(),
		int(chainID),
		tokenAddr,
		recipientAddress,
	)
	if err != nil {
		return "", err
	}
	if hashes == nil {
		hashes = []string{}
	}
	return codec.JSONString(hashes)
}

func Swap(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON, actionID, swapData, feeToken, feeStructureJSON string) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	return h.Swap(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		types.ExternalActionID(actionID),
		swapData,
		feeToken,
		feeOverride,
	)
}

func SwapSolana(h *core.Hinkal, chainID int64, tokenAddrsJSON, amountsWeiJSON, swapData, feeToken, feeStructureJSON string) (string, error) {
	tokenAddrs, amounts, err := codec.DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON)
	if err != nil {
		return "", err
	}
	feeOverride, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	return h.SwapSolana(
		context.Background(),
		int(chainID),
		tokenAddrs,
		amounts,
		swapData,
		feeToken,
		feeOverride,
	)
}

func EmporiumOp(h *core.Hinkal, contract, callDataString string, invokeWallet bool, valueWei string) (string, error) {
	var value *big.Int
	if valueWei != "" {
		var err error
		value, err = codec.DecodeBig(valueWei)
		if err != nil {
			return "", err
		}
	}
	return h.EmporiumOp(contract, callDataString, invokeWallet, value)
}

func BridgePrivateToPrivate(h *core.Hinkal, claimable map[string]*utxo.Utxo, sourceChainID int64, sourceTokenAddr string, destChainID int64, destTokenAddr, amount, recipientJSON string, slippage float64, feeToken string) (string, error) {
	recipient, err := codec.DecodePrivateBridgeRecipient(recipientJSON)
	if err != nil {
		return "", err
	}
	res, err := h.BridgePrivateToPrivate(
		context.Background(),
		int(sourceChainID),
		sourceTokenAddr,
		int(destChainID),
		destTokenAddr,
		amount,
		recipient,
		slippage,
		feeToken,
	)
	if err != nil {
		return "", err
	}
	out := mobiletypes.PrivateBridgeResultJSON{
		SourceTxHash: res.SourceTxHash,
		DestTxHash:   res.DestTxHash,
	}
	if pending := res.PendingEnclaveUtxo; pending != nil && pending.Utxo != nil {
		entry, err := registerClaimable(claimable, pending.Utxo)
		if err != nil {
			return "", err
		}
		out.PendingEnclaveUtxo = &mobiletypes.PendingEnclaveUtxoJSON{
			ChainID:            pending.ChainID,
			SenderAddress:      pending.SenderAddress,
			RecipientAddress:   pending.RecipientAddress,
			ClaimableSignature: pending.ClaimableSignature,
			Utxo:               entry,
		}
	}
	return codec.JSONString(out)
}
