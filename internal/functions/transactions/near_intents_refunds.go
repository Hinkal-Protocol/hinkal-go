package transactions

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mr-tron/base58"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type nearCallFunctionResponse struct {
	Result *struct {
		Result []int  `json:"result"`
		Error  string `json:"error"`
	} `json:"result"`
	Error json.RawMessage `json:"error"`
}

type nearRPCHTTPError struct {
	status int
}

func (e nearRPCHTTPError) Error() string {
	return fmt.Sprintf("Request failed with status code %d", e.status)
}

func isRetryableNearRPCError(err error) bool {
	var httpErr nearRPCHTTPError
	return !errors.As(err, &httpErr) || slices.Contains(constants.NearRPCRetryStatuses, httpErr.status)
}

func postNearRPCRequest(ctx context.Context, body any) (nearCallFunctionResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nearCallFunctionResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, constants.NearRPCURL, bytes.NewReader(payload))
	if err != nil {
		return nearCallFunctionResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client.Do(req)
	if err != nil {
		return nearCallFunctionResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nearCallFunctionResponse{}, nearRPCHTTPError{status: resp.StatusCode}
	}
	var response nearCallFunctionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nearCallFunctionResponse{}, err
	}
	return response, nil
}

func postNearRPC(ctx context.Context, body any, attempt int) (nearCallFunctionResponse, error) {
	response, err := postNearRPCRequest(ctx, body)
	if err == nil {
		return response, nil
	}
	if attempt >= constants.NearRPCMaxRetryAttempts || !isRetryableNearRPCError(err) {
		return nearCallFunctionResponse{}, err
	}
	if err := wait(ctx, time.Duration(constants.NearRPCRetryDelayMS*math.Pow(constants.NearRPCRetryBackoff, float64(attempt-1)))*time.Millisecond); err != nil {
		return nearCallFunctionResponse{}, err
	}
	return postNearRPC(ctx, body, attempt+1)
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func callFunction[T any](ctx context.Context, contractID string, method string, args any) (*T, error) {
	encodedArgs, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	response, err := postNearRPC(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      "dontcare",
		"method":  "query",
		"params": map[string]any{
			"request_type": "call_function",
			"finality":     "optimistic",
			"account_id":   contractID,
			"method_name":  method,
			"args_base64":  base64.StdEncoding.EncodeToString(encodedArgs),
		},
	}, 1)
	if err != nil {
		return nil, err
	}
	hasError := len(response.Error) > 0 && string(response.Error) != "null"
	if hasError || response.Result == nil || response.Result.Error != "" || response.Result.Result == nil {
		return nil, fmt.Errorf("NEAR RPC call %s failed", method)
	}
	if len(response.Result.Result) == 0 {
		return nil, nil
	}
	raw := make([]byte, len(response.Result.Result))
	for i, value := range response.Result.Result {
		raw[i] = byte(value)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

var (
	sdkInstance     *intentsSDK
	sdkInstanceOnce sync.Once
)

func getSdk() *intentsSDK {
	sdkInstanceOnce.Do(func() {
		sdkInstance = newIntentsSDK(constants.NearIntentsReferral)
	})
	return sdkInstance
}

func buildIntentSigner(accountID string, keyPairString string) (*intentSignerNEP413, error) {
	encodedSecretKey, ok := strings.CutPrefix(keyPairString, "ed25519:")
	if !ok {
		return nil, errors.New("key pair string must start with ed25519:")
	}
	secretKey, err := base58.Decode(encodedSecretKey)
	if err != nil {
		return nil, err
	}
	if len(secretKey) != ed25519.PrivateKeySize {
		return nil, errors.New("bad secret key size")
	}
	return &intentSignerNEP413{accountID: accountID, publicKey: "ed25519:" + base58.Encode(secretKey[32:]), secretKey: secretKey}, nil
}

type nearBatchBalanceOfArgs struct {
	AccountID string   `json:"account_id"`
	TokenIDs  []string `json:"token_ids"`
}

func fetchAccountRefundBalances(
	ctx context.Context,
	accountID string,
	tokens []types.NearIntentsToken,
	isLegacy bool,
) ([]types.NearIntentsRefundBalance, error) {
	tokenIDs := make([]string, len(tokens))
	for i, token := range tokens {
		tokenIDs[i] = token.AssetID
	}
	if len(tokenIDs) == 0 {
		return nil, nil
	}

	rawBalances, err := callFunction[[]string](ctx, constants.NearIntentsContractID, "mt_batch_balance_of", nearBatchBalanceOfArgs{
		AccountID: accountID,
		TokenIDs:  tokenIDs,
	})
	if err != nil {
		return nil, err
	}
	if rawBalances == nil {
		return nil, nil
	}

	refundCandidates := make([]*types.NearIntentsRefundBalance, len(*rawBalances))
	group, groupCtx := errgroup.WithContext(ctx)
	for index, rawBalance := range *rawBalances {
		index, rawBalance := index, rawBalance
		group.Go(func() error {
			balance, ok := new(big.Int).SetString(rawBalance, 10)
			if !ok {
				return fmt.Errorf("Cannot convert %s to a BigInt", rawBalance)
			}
			if balance.Sign() <= 0 {
				return nil
			}

			token, err := integrations.GetErc20TokenForNearAsset(groupCtx, tokens[index])
			if err != nil {
				return err
			}
			if token == nil {
				return nil
			}

			refundCandidates[index] = &types.NearIntentsRefundBalance{
				TokenBalanceWithUsdValue: types.TokenBalanceWithUsdValue{Token: *token, Balance: balance},
				AssetID:                  tokens[index].AssetID,
				IsLegacy:                 isLegacy,
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	refunds := make([]types.NearIntentsRefundBalance, 0, len(refundCandidates))
	for _, refund := range refundCandidates {
		if refund != nil {
			refunds = append(refunds, *refund)
		}
	}
	return refunds, nil
}

// fetchNearIntentsRefundBalances reads the cross-chain bridge refunds resting in the user's NEAR Intents account. Refunds land here
// (refundType INTENTS, see hinkalNearDepositAndBridge) when a bridge leg fails, instead of leaking
// the user's public main wallet. Checks both the current and legacy (old nk-derived) account and
// tags each balance so claimNearIntentsRefund signs with the matching key set.
func fetchNearIntentsRefundBalances(ctx context.Context, userKeys *cryptokeys.UserKeys) ([]types.NearIntentsRefundBalance, error) {
	tokens, err := api.GetNearIntentsTokens(ctx)
	if err != nil {
		return nil, err
	}

	currentAccountID, err := userKeys.GetNearIntentsAccountID()
	if err != nil {
		return nil, err
	}
	legacyAccountID, err := userKeys.GetLegacyNearIntentsAccountID()
	if err != nil {
		return nil, err
	}

	var current, legacy []types.NearIntentsRefundBalance
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		current, err = fetchAccountRefundBalances(groupCtx, currentAccountID, tokens, false)
		return err
	})
	group.Go(func() (err error) {
		legacy, err = fetchAccountRefundBalances(groupCtx, legacyAccountID, tokens, true)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	return append(current, legacy...), nil
}

type claimNearIntentsRefundParams struct {
	UserKeys           *cryptokeys.UserKeys
	AssetID            string
	Amount             *big.Int
	DestinationAddress string
	IsLegacy           bool
}

// startNearIntentsRefundClaim claims a refund out of the user's NEAR Intents account to a destination wallet on its origin chain.
// Signs a NEP-413 withdrawal intent with the shielded-derived key (no wallet popup, no user gas), then
// waits for the POA bridge to settle and release the real tokens. The returned wait resolves only once the withdrawal has
// finalized, so a balance refetch afterwards reflects the now-zero Intents balance. Fee is deducted from
// the amount.
// The intent is published before returning; the wait is returned so a caller holding a lock can release it first.
func startNearIntentsRefundClaim(ctx context.Context, params claimNearIntentsRefundParams) (func(context.Context) error, error) {
	var accountID, keyPairString string
	var err error
	if params.IsLegacy {
		accountID, err = params.UserKeys.GetLegacyNearIntentsAccountID()
	} else {
		accountID, err = params.UserKeys.GetNearIntentsAccountID()
	}
	if err != nil {
		return nil, err
	}
	if params.IsLegacy {
		keyPairString, err = params.UserKeys.GetLegacyNearIntentsKeyPairString()
	} else {
		keyPairString, err = params.UserKeys.GetNearIntentsKeyPairString()
	}
	if err != nil {
		return nil, err
	}

	intentSigner, err := buildIntentSigner(accountID, keyPairString)
	if err != nil {
		return nil, err
	}
	sdk := getSdk()
	withdrawal := withdrawalParams{
		AssetID:            params.AssetID,
		Amount:             params.Amount,
		DestinationAddress: params.DestinationAddress,
		FeeInclusive:       true,
	}
	intentHash, err := sdk.submitWithdrawal(ctx, intentSigner, withdrawal)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return sdk.awaitWithdrawal(ctx, withdrawal, intentHash)
	}, nil
}
