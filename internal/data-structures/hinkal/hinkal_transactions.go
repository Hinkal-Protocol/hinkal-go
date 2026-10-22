package hinkal

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/receivevault"
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
	return transactions.HinkalDeposit(ctx, h, erc20Tokens, amountChanges, preEstimateGas, returnTxData, pretransaction.SdkAction(pretransaction.SdkDeposit))
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
	return transactions.HinkalDepositForOther(ctx, h, erc20Tokens, amountChanges, recipientInfo, preEstimateGas, returnTxData, pretransaction.SdkAction(pretransaction.SdkDepositForOther))
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
	return transactions.HinkalSolanaDeposit(ctx, h, amount, token, returnTxData, pretransaction.SdkAction(pretransaction.SdkDepositSolana))
}

func (h *Hinkal) DepositSolanaForOther(
	ctx context.Context,
	chainID int,
	erc20Address string,
	amount *big.Int,
	recipientInfo string,
	returnTxData bool,
	variableRate *big.Int,
) (string, error) {
	if err := h.enforceRateLimit("DepositSolanaForOther", amount, erc20Address, recipientInfo); err != nil {
		return "", err
	}
	token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaDepositForOther(ctx, h, amount, token, recipientInfo, returnTxData, variableRate, pretransaction.SdkAction(pretransaction.SdkDepositSolanaForOther))
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
		txDataOrSignature, err := transactions.HinkalSolanaProoflessDeposit(ctx, h, erc20Tokens, amountChanges, stealthAddressStructures, recipientEncryptionKeysOverride, returnTxData, pretransaction.SdkAction(pretransaction.SdkProoflessDeposit))
		return types.TransactionRequest{}, txDataOrSignature, err
	}
	return transactions.HinkalProoflessDeposit(ctx, h, erc20Tokens, amountChanges, stealthAddressStructures, recipientEncryptionKeysOverride, createBlockedUtxos, feeStructure, orderID, returnTxData, pretransaction.SdkAction(pretransaction.SdkProoflessDeposit))
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
	action types.AdminTransactionType,
) (types.TransactionRequest, string, error) {
	if action == "" {
		action = pretransaction.SdkAction(pretransaction.SdkWithdraw)
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		txHash, err := transactions.HinkalSolanaWithdraw(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeStructureOverride, nil, action)
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
		nil,
		action,
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
		return transactions.HinkalSolanaTransfer(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeStructureOverride, pretransaction.SdkAction(pretransaction.SdkTransfer))
	}
	return transactions.HinkalTransfer(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride, pretransaction.SdkAction(pretransaction.SdkTransfer))
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
			pretransaction.SdkAction(pretransaction.SdkDepositAndWithdraw),
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
		pretransaction.SdkAction(pretransaction.SdkDepositAndWithdraw),
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
		pretransaction.SdkAction(pretransaction.SdkDepositAndBridge),
	)
}

func (h *Hinkal) BridgePrivateToPrivate(
	ctx context.Context,
	sourceChainID int,
	sourceTokenAddress string,
	destChainID int,
	destTokenAddress string,
	amount string,
	vaultAddress string,
	slippage float64,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (bridging.PrivateBridgeResult, error) {
	if err := h.enforceRateLimit("BridgePrivateToPrivate", sourceChainID, sourceTokenAddress, destChainID, destTokenAddress, amount, vaultAddress); err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	sourceToken, err := resolveERC20Token(ctx, sourceChainID, sourceTokenAddress)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	destToken, err := resolveERC20Token(ctx, destChainID, destTokenAddress)
	if err != nil {
		return bridging.PrivateBridgeResult{}, err
	}
	return transactions.HinkalBridgePrivateToPrivate(ctx, h, sourceToken, destToken, amount, vaultAddress, slippage, feeToken, feeStructureOverride, pretransaction.SdkAction(pretransaction.SdkBridgePrivateToPrivate))
}

func (h *Hinkal) ProxyToPrivate(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	amount *big.Int,
	subAccount types.TemporarySubAccount,
	recipientInfo string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	token, err := resolveERC20Token(ctx, chainID, tokenAddress)
	if err != nil {
		return "", err
	}
	return transactions.HinkalProxyToPrivate(ctx, h, chainID, token, amount, subAccount, recipientInfo, feeToken, feeStructureOverride, pretransaction.SdkAction(pretransaction.SdkProxyToPrivate))
}

func (h *Hinkal) SolanaSponsoredShield(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	tempSubAccount types.TemporarySubAccount,
	landedAmount *big.Int,
	recipientInfo string,
) (string, *big.Int, error) {
	token, err := resolveERC20Token(ctx, chainID, tokenAddress)
	if err != nil {
		return "", nil, err
	}
	return transactions.HinkalSolanaSponsoredShield(ctx, h, token, tempSubAccount, landedAmount, recipientInfo, pretransaction.SdkAction(pretransaction.SdkSolanaSponsoredShield), nil)
}

func (h *Hinkal) SolanaSponsoredProxySend(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	tempSubAccount types.TemporarySubAccount,
	landedAmount *big.Int,
	recipientAddress string,
) (string, error) {
	token, err := resolveERC20Token(ctx, chainID, tokenAddress)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaSponsoredProxySend(ctx, h, token, tempSubAccount, landedAmount, recipientAddress, pretransaction.SdkAction(pretransaction.SdkSolanaSponsoredProxySend))
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
		pretransaction.SdkAction(pretransaction.SdkNearDepositAndBridge),
	)
}

func (h *Hinkal) GetDepositAndBridgeQuote(
	ctx context.Context,
	chainID int,
	erc20Address string,
	destinationChainID int,
	destinationErc20Address string,
	sendAmounts []*big.Int,
	recipientAddresses []string,
	slippagePercent *float64,
	txCompletionTime *int,
) (types.DepositAndBridgeQuote, error) {
	var sourceToken, destinationToken types.ERC20Token
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		sourceToken, err = resolveERC20Token(groupCtx, chainID, erc20Address)
		return err
	})
	group.Go(func() (err error) {
		destinationToken, err = resolveERC20Token(groupCtx, destinationChainID, destinationErc20Address)
		return err
	})
	if err := group.Wait(); err != nil {
		return types.DepositAndBridgeQuote{}, err
	}

	return transactions.GetDepositAndBridgeQuote(
		ctx,
		h,
		sourceToken,
		destinationToken,
		sendAmounts,
		recipientAddresses,
		slippagePercent,
		txCompletionTime,
	)
}

func (h *Hinkal) DepositAndBridgeWithQuote(
	ctx context.Context,
	chainID int,
	erc20Address string,
	quote types.DepositAndBridgeQuote,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
) (types.DepositAndSendExtendedResult, error) {
	if err := h.enforceRateLimit("DepositAndBridgeWithQuote", erc20Address, quote); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return transactions.HinkalDepositAndBridgeWithQuote(
		ctx,
		h,
		token,
		quote,
		txCompletionTime,
		feeStructureOverride,
		pretransaction.SdkAction(pretransaction.SdkDepositAndBridgeWithQuote),
	)
}

func (h *Hinkal) GetStuckBridgeBalances(ctx context.Context, chainIDs []int) []types.StuckBridgeBalance {
	return transactions.GetStuckBridgeBalances(ctx, h, chainIDs)
}

func (h *Hinkal) RecoverStuckBridgeBalance(ctx context.Context, balance types.StuckBridgeBalance, recipientAddress string) error {
	return transactions.RecoverStuckBridgeBalance(ctx, h, balance, recipientAddress)
}

func (h *Hinkal) StartRecoverStuckBridgeBalance(
	ctx context.Context,
	balance types.StuckBridgeBalance,
	recipientAddress string,
) (func(context.Context) error, error) {
	return transactions.StartStuckBridgeBalanceRecovery(ctx, h, balance, recipientAddress)
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

func (h *Hinkal) CreateReceiveAddress(ctx context.Context, chainID int, forceFresh ...bool) (types.ReceiveVaultRecord, error) {
	return receivevault.HinkalCreateReceiveAddress(ctx, h, chainID, forceFresh...)
}

func (h *Hinkal) GetReceiveVaultAccount(ctx context.Context) (types.ReceiveVaultAccount, error) {
	return receivevault.HinkalGetReceiveVaultAccount(ctx, h)
}

func (h *Hinkal) RecoverReceiveVault(
	ctx context.Context,
	record types.ReceiveVaultRecord,
	token string,
	to string,
	returnTxData bool,
	chainID int,
) (types.TransactionRequest, string, error) {
	return receivevault.HinkalReceiveVaultRecover(ctx, h, record, token, to, returnTxData, chainID)
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
	slippagePercentage float64, // percent, e.g. 0.7; 0 uses DefaultBridgingSlippage
) (string, error) {
	if constants.IsSolanaLike(chainID) {
		return h.SwapSolana(ctx, chainID, erc20Addresses, deltaAmounts, swapData, feeToken, feeStructureOverride, slippagePercentage)
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSwap(ctx, h, erc20Tokens, deltaAmounts, externalActionID, swapData, feeToken, feeStructureOverride, slippagePercentage, pretransaction.SdkAction(pretransaction.SdkSwap))
}

// feeToken is unused on Solana (fee is always paid in the output token) and kept for API compatibility.
func (h *Hinkal) SwapSolana(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	deltaAmounts []*big.Int,
	swapData string,
	_feeToken string,
	feeStructureOverride *types.FeeStructure,
	slippagePercentage float64, // percent, e.g. 0.7; 0 uses DefaultBridgingSlippage
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
		feeStructureOverride,
		slippagePercentage,
		pretransaction.SdkAction(pretransaction.SdkSwap),
	)
}
