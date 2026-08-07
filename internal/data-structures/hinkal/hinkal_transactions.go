package hinkal

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/transactions"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

var errInvalidSwapperAccountSalt = errors.New("hinkal: invalid swapperAccountSalt in swap data")

func resolveERC20Tokens(ctx context.Context, chainID int, erc20Addresses []string) ([]types.ERC20Token, error) {
	return web3.ResolveERC20TokensStrict(ctx, chainID, erc20Addresses)
}

func resolveERC20Token(ctx context.Context, chainID int, erc20Address string) (types.ERC20Token, error) {
	return web3.ResolveERC20TokenStrict(ctx, chainID, erc20Address)
}

func (h *Hinkal) GetRecipientInfo() (string, error) {
	return pretransaction.GetRecipientInfoFromUserKeys(h.UserKeys)
}

func (h *Hinkal) GetFeeStructure(
	ctx context.Context,
	chainID int,
	feeTokenAddress string,
	erc20Addresses []string,
	externalActionID types.ExternalActionID,
	calls []types.CallInfo,
	variableRate *big.Int,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	return pretransaction.GetFeeStructure(ctx, chainID, feeTokenAddress, erc20Addresses, externalActionID, calls, variableRate, solanaTransactionParams)
}

func (h *Hinkal) EmporiumOp(contract, callDataString string, invokeWallet bool, value *big.Int) (string, error) {
	return privatewallet.EmporiumOp(contract, callDataString, invokeWallet, value)
}

func (h *Hinkal) GetEvmSwapPrices(ctx context.Context, chainID int, inSwapAmount, inSwapTokenAddress, outSwapTokenAddress string) (*integrations.EVMSwapPrice, error) {
	return integrations.GetEVMSwapPrices(ctx, chainID, inSwapAmount, inSwapTokenAddress, outSwapTokenAddress)
}

func (h *Hinkal) GetSolanaSwapPrices(ctx context.Context, chainID int, inSwapAmount, inSwapTokenAddress, outSwapTokenAddress string) (*integrations.SolanaSwapPrice, error) {
	return integrations.GetSolanaSwapPrices(ctx, chainID, inSwapAmount, inSwapTokenAddress, outSwapTokenAddress)
}

func (h *Hinkal) Deposit(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	preEstimateGas bool,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return transactions.HinkalDeposit(ctx, h, erc20Tokens, amountChanges, preEstimateGas, returnTxData)
}

func (h *Hinkal) DepositForOther(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientInfo string,
	preEstimateGas bool,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	if err := h.enforceRateLimit("DepositForOther", erc20Addresses, amountChanges, recipientInfo); err != nil {
		return types.TransactionRequest{}, "", err
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return transactions.HinkalDepositForOther(ctx, h, erc20Tokens, amountChanges, recipientInfo, preEstimateGas, returnTxData)
}

func (h *Hinkal) DepositSolana(
	ctx context.Context,
	chainID int,
	erc20Address string,
	amount *big.Int,
	returnTxData bool,
) (string, error) {
	token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaDeposit(ctx, h, amount, token, returnTxData)
}

func (h *Hinkal) DepositSolanaForOther(
	ctx context.Context,
	chainID int,
	erc20Address string,
	amount *big.Int,
	recipientInfo string,
	returnTxData bool,
) (string, error) {
	if err := h.enforceRateLimit("DepositSolanaForOther", amount, erc20Address, recipientInfo); err != nil {
		return "", err
	}
	token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaDepositForOther(ctx, h, amount, token, recipientInfo, returnTxData)
}

func (h *Hinkal) ProoflessDeposit(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	stealthAddressStructures []types.StealthAddressStructure,
	recipientEncryptionKeysOverride []string,
	createBlockedUtxos bool,
	feeStructure types.ProoflessFeeStructure,
	orderID string,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		txDataOrSignature, err := transactions.HinkalSolanaProoflessDeposit(ctx, h, erc20Tokens, amountChanges, stealthAddressStructures, returnTxData)
		return types.TransactionRequest{}, txDataOrSignature, err
	}
	return transactions.HinkalProoflessDeposit(ctx, h, erc20Tokens, amountChanges, stealthAddressStructures, recipientEncryptionKeysOverride, createBlockedUtxos, feeStructure, orderID, returnTxData)
}

func (h *Hinkal) Withdraw(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	isRelayerOff bool,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		txHash, err := transactions.HinkalSolanaWithdraw(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride)
		return types.TransactionRequest{}, txHash, err
	}
	return transactions.HinkalWithdraw(
		ctx,
		h,
		erc20Tokens,
		amountChanges,
		recipientAddress,
		isRelayerOff,
		feeToken,
		feeStructureOverride,
	)
}

func (h *Hinkal) Transfer(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	if err := h.enforceRateLimit("Transfer", erc20Addresses, amountChanges, recipientAddress); err != nil {
		return "", err
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	if constants.IsSolanaLike(chainID) {
		return transactions.HinkalSolanaTransfer(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride)
	}
	return transactions.HinkalTransfer(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride)
}

func (h *Hinkal) ClaimUtxo(
	ctx context.Context,
	chainID int,
	erc20Address string,
	claimableUtxo *utxo.Utxo,
	feeStructureOverride *types.FeeStructure,
	claimableSignature string,
) (string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return "", err
	}
	if constants.IsSolanaLike(chainID) {
		return transactions.HinkalSolanaClaimUtxo(ctx, h, erc20Tokens, claimableUtxo, feeStructureOverride, claimableSignature)
	}
	return transactions.HinkalClaimUtxo(ctx, h, erc20Tokens, claimableUtxo, feeStructureOverride, claimableSignature)
}

func (h *Hinkal) DepositAndWithdraw(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if constants.IsSolanaLike(chainID) {
		return transactions.HinkalSolanaDepositAndWithdraw(
			ctx,
			h,
			erc20Tokens,
			recipientAmounts,
			recipientAddresses,
			txCompletionTime,
			feeStructureOverride,
		)
	}
	return transactions.HinkalDepositAndWithdraw(
		ctx,
		h,
		erc20Tokens,
		recipientAmounts,
		recipientAddresses,
		txCompletionTime,
		feeStructureOverride,
		preEstimateGas,
	)
}

func (h *Hinkal) DepositAndBridge(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipients []types.BridgeRecipient,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return transactions.HinkalDepositAndBridge(
		ctx,
		h,
		erc20Tokens,
		recipients,
		txCompletionTime,
		feeStructureOverride,
		preEstimateGas,
	)
}

func (h *Hinkal) BridgePrivateToPrivate(
	ctx context.Context,
	sourceChainID int,
	sourceTokenAddress string,
	destChainID int,
	destTokenAddress string,
	amount string,
	recipient bridging.PrivateBridgeRecipient,
	slippage float64,
	feeToken string,
) (bridging.PrivateBridgeResult, error) {
	sourceToken, err := resolveERC20Token(ctx, sourceChainID, sourceTokenAddress)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	destToken, err := resolveERC20Token(ctx, destChainID, destTokenAddress)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	return transactions.HinkalBridgePrivateToPrivate(ctx, h, sourceToken, destToken, amount, recipient, slippage, feeToken)
}

func (h *Hinkal) NearDepositAndBridge(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	params types.NearBridgeParams,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
) (types.NearBridgeResult, error) {
	if err := h.enforceRateLimit("NearDepositAndBridge", erc20Address, recipientAmounts, recipientAddresses, params); err != nil {
		return types.NearBridgeResult{}, err
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.NearBridgeResult{}, err
	}
	return transactions.HinkalNearDepositAndBridge(
		ctx,
		h,
		erc20Tokens,
		recipientAmounts,
		recipientAddresses,
		params,
		txCompletionTime,
		feeStructureOverride,
	)
}

func (h *Hinkal) WithdrawStuckUtxos(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipientAddress string,
) ([]string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return nil, err
	}
	return transactions.HinkalWithdrawStuckUtxos(ctx, h, erc20Tokens, recipientAddress)
}

func (h *Hinkal) CheckSendTransactionStatus(ctx context.Context, scheduleID string) (types.ScheduledTransactionByIDResponse, error) {
	return api.GetScheduledTransactionByID(ctx, scheduleID)
}

func (h *Hinkal) Swap(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	deltaAmounts []*big.Int,
	externalActionID types.ExternalActionID,
	swapData string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	if constants.IsSolanaLike(chainID) {
		return h.SwapSolana(ctx, chainID, erc20Addresses, deltaAmounts, swapData, feeToken, feeStructureOverride)
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSwap(ctx, h, erc20Tokens, deltaAmounts, externalActionID, swapData, feeToken, feeStructureOverride)
}

func (h *Hinkal) SwapSolana(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	deltaAmounts []*big.Int,
	swapData string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	var parsed struct {
		api.OKXSwapResponse
		SwapperAccountSalt string `json:"swapperAccountSalt"`
	}
	if err := json.Unmarshal([]byte(swapData), &parsed); err != nil {
		return "", err
	}
	salt, ok := new(big.Int).SetString(parsed.SwapperAccountSalt, 10)
	if !ok {
		return "", errInvalidSwapperAccountSalt
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaSwap(
		ctx,
		h,
		erc20Tokens,
		deltaAmounts,
		salt,
		parsed.Data.InstructionLists,
		parsed.Data.AddressLookupTableAccount,
		feeToken,
		feeStructureOverride,
	)
}
