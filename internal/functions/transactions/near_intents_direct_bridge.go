package transactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// ---------------------------------------------------------------------------------------------------
// intents-sdk: DirectBridge (bridges/direct-bridge). Catches every nep141 asset PoA and Omni do not and
// withdraws it to a NEAR account. Hinkal never passes routeConfig, so routeConfig.msg is always undefined.
// ---------------------------------------------------------------------------------------------------

const (
	routeNearWithdrawal = "near_withdrawal"
	caip2Near           = "near:mainnet"
	directMinGasAmount  = "17050000000000"
)

type directBridge struct {
	storageDepositCache   *ttlCache[[2]*big.Int]
	accountExistenceCache *ttlCache[bool]
}

func newDirectBridge() *directBridge {
	return &directBridge{
		storageDepositCache:   newTTLCache[[2]*big.Int](time.Hour),
		accountExistenceCache: newTTLCache[bool](time.Hour),
	}
}

func (b *directBridge) route() string { return routeNearWithdrawal }

func (b *directBridge) supports(_ context.Context, params withdrawalParams) (bool, error) {
	parsed, err := parseDefuseAssetID(params.AssetID)
	if err != nil {
		return false, err
	}
	return parsed.standard == "nep141", nil
}

func (b *directBridge) createWithdrawalIntents(params withdrawalParams, fee feeEstimation, referral string) ([]any, error) {
	intents := []any{}
	if fee.quote != nil {
		intents = append(intents, struct {
			Intent   string           `json:"intent"`
			Diff     orderedTokenDiff `json:"diff"`
			Referral string           `json:"referral,omitempty"`
		}{
			Intent: "token_diff",
			Diff: orderedTokenDiff{
				assetIn:   fee.quote.DefuseAssetIdentifierIn,
				amountIn:  "-" + fee.quote.AmountIn,
				assetOut:  fee.quote.DefuseAssetIdentifierOut,
				amountOut: fee.quote.AmountOut,
			},
			Referral: referral,
		})
	}

	storageDeposit, err := getUnderlyingFee(fee, routeNearWithdrawal, "storageDepositFee")
	if err != nil {
		return nil, err
	}

	// createWithdrawIntentPrimitive (direct-bridge-utils.js)
	if params.AssetID == nearNativeAssetID {
		return append(intents, struct {
			Intent     string `json:"intent"`
			ReceiverID string `json:"receiver_id"`
			Amount     string `json:"amount"`
		}{
			Intent:     "native_withdraw",
			ReceiverID: params.DestinationAddress,
			Amount:     params.Amount.String(),
		}), nil
	}
	parsed, err := parseDefuseAssetID(params.AssetID)
	if err != nil {
		return nil, err
	}
	if parsed.standard != "nep141" {
		return nil, errors.New("Only NEP-141 is supported")
	}
	storageDepositValue := ""
	if storageDeposit != nil && storageDeposit.Sign() > 0 {
		storageDepositValue = storageDeposit.String()
	}
	return append(intents, struct {
		Intent         string `json:"intent"`
		Token          string `json:"token"`
		ReceiverID     string `json:"receiver_id"`
		Amount         string `json:"amount"`
		StorageDeposit string `json:"storage_deposit,omitempty"`
		MinGas         string `json:"min_gas"`
	}{
		Intent:         "ft_withdraw",
		Token:          parsed.contractID,
		ReceiverID:     params.DestinationAddress,
		Amount:         params.Amount.String(),
		StorageDeposit: storageDepositValue,
		MinGas:         directMinGasAmount,
	}), nil
}

func (b *directBridge) validateWithdrawal(ctx context.Context, _ string, _ *big.Int, destinationAddress string, _ feeEstimation) error {
	if !validateNearAddress(destinationAddress) {
		return fmt.Errorf("Invalid destination address. Details: Destination address %s is not valid for withdrawal to %s.", destinationAddress, caip2Near)
	}
	if isImplicitAccount(destinationAddress) {
		return nil
	}
	exists, err := b.getCachedAccountExistenceCheck(ctx, destinationAddress)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Destination explicit NEAR account doesn't exist. Account Id: %s", destinationAddress)
	}
	return nil
}

func (b *directBridge) estimateWithdrawalFee(ctx context.Context, params withdrawalParams) (feeEstimation, error) {
	zeroFee := feeEstimation{
		amount:         big.NewInt(0),
		underlyingFees: map[string]map[string]*big.Int{routeNearWithdrawal: {"storageDepositFee": big.NewInt(0)}},
	}
	parsed, err := parseDefuseAssetID(params.AssetID)
	if err != nil {
		return feeEstimation{}, err
	}
	if parsed.standard != "nep141" {
		return feeEstimation{}, errors.New("Only NEP-141 is supported")
	}
	if params.AssetID == nearNativeAssetID {
		return zeroFee, nil
	}

	storage, err := b.getCachedStorageDepositValue(ctx, parsed.contractID, params.DestinationAddress)
	if err != nil {
		return feeEstimation{}, err
	}
	minStorageBalance, userStorageBalance := storage[0], storage[1]
	if minStorageBalance.Cmp(userStorageBalance) <= 0 {
		return zeroFee, nil
	}

	feeAssetID := nearNativeAssetID
	feeAmount := new(big.Int).Sub(minStorageBalance, userStorageBalance)
	underlyingFees := map[string]map[string]*big.Int{routeNearWithdrawal: {"storageDepositFee": feeAmount}}
	if params.AssetID == feeAssetID {
		return feeEstimation{amount: feeAmount, underlyingFees: underlyingFees}, nil
	}
	feeQuote, err := getFeeQuote(ctx, feeAmount, feeAssetID, params.AssetID)
	if err != nil {
		return feeEstimation{}, err
	}
	return feeEstimation{amount: mustBigInt(feeQuote.AmountIn), quote: &feeQuote, underlyingFees: underlyingFees}, nil
}

func (b *directBridge) getCachedStorageDepositValue(ctx context.Context, contractID string, accountID string) ([2]*big.Int, error) {
	key := contractID + accountID
	if cached, ok := b.storageDepositCache.get(key); ok {
		return cached, nil
	}
	result, err := getNearNep141StorageBalances(ctx, contractID, accountID)
	if err != nil {
		return [2]*big.Int{}, err
	}
	if result[1].Cmp(result[0]) >= 0 {
		b.storageDepositCache.set(key, result)
	}
	return result, nil
}

// getCachedAccountExistenceCheck caches only accounts that exist.
func (b *directBridge) getCachedAccountExistenceCheck(ctx context.Context, accountID string) (bool, error) {
	if cached, ok := b.accountExistenceCache.get(accountID); ok {
		return cached, nil
	}
	exists, err := accountExistsInNEAR(ctx, accountID)
	if err != nil {
		return false, err
	}
	if exists {
		b.accountExistenceCache.set(accountID, true)
	}
	return exists, nil
}

// accountExistsInNEAR (direct-bridge-utils.js): view_account; UNKNOWN_ACCOUNT (near-api-js AccountDoesNotExist) means false.
func accountExistsInNEAR(ctx context.Context, accountID string) (bool, error) {
	response, err := postNearRPC(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      "dontcare",
		"method":  "query",
		"params": map[string]any{
			"request_type": "view_account",
			"finality":     "final",
			"account_id":   accountID,
		},
	}, 1)
	if err != nil {
		return false, err
	}
	if len(response.Error) == 0 || string(response.Error) == "null" {
		return true, nil
	}
	var rpcError struct {
		Message string `json:"message"`
		Cause   struct {
			Name string `json:"name"`
		} `json:"cause"`
	}
	if err := json.Unmarshal(response.Error, &rpcError); err != nil {
		return false, fmt.Errorf("NEAR RPC view_account failed: %s", string(response.Error))
	}
	if rpcError.Cause.Name == "UNKNOWN_ACCOUNT" {
		return false, nil
	}
	return false, fmt.Errorf("NEAR RPC view_account failed: %s", rpcError.Message)
}

func (b *directBridge) createWithdrawalIdentifier(params withdrawalParams, index int, txHash string) (withdrawalIdentifier, error) {
	return withdrawalIdentifier{landingChain: caip2Near, index: index, withdrawalParams: params, txHash: txHash}, nil
}

func (b *directBridge) describeWithdrawal(_ context.Context, _ withdrawalIdentifier) (withdrawalStatus, error) {
	return withdrawalStatus{status: "completed"}, nil
}
