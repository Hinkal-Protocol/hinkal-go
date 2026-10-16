package transactions

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	solana "github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"github.com/mr-tron/base58"
	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

// Port of IntentsSDK.processWithdrawal from @defuse-protocol/intents-sdk 0.48.0 (env "production"), with the
// parts of @defuse-protocol/internal-utils 0.25.0 and omni-bridge-sdk 0.23.1 it runs, for the bridges a
// NEAR Intents refund withdrawal resolves to: PoaBridge, HotBridge (EVM chains), OmniBridge and DirectBridge.

// configsByEnvironment.production (internal-utils config.js)
const (
	envPoaTokenFactoryContractID = "omft.near"
	envPoaBridgeBaseURL          = "https://bridge.chaindefuser.com"
	envSolverRelayBaseURL        = "https://solver-relay-v2.chaindefuser.com"
	envManagerConsoleBaseURL     = "https://api-mng-console.chaindefuser.com/api/"
)

// RouteEnum (constants/route-enum.js)
const (
	routePoaBridge  = "poa_bridge"
	routeOmniBridge = "omni_bridge"
)

// Chains (lib/caip2.js)
const (
	caip2Bitcoin     = "bip122:000000000019d6689c085ae165831e93"
	caip2BitcoinCash = "bip122:000000000000000000651ef99cb9fcbe"
	caip2Zcash       = "bip122:00040fe8ec8471911baa1db1266ea15d"
	caip2Dogecoin    = "bip122:1a91e3dace36e2be3bf030a65679fe82"
	caip2Litecoin    = "bip122:12a765e31ffd4059bada1e25190f6e98"
	caip2Ethereum    = "eip155:1"
	caip2Optimism    = "eip155:10"
	caip2BNB         = "eip155:56"
	caip2Gnosis      = "eip155:100"
	caip2Polygon     = "eip155:137"
	caip2Monad       = "eip155:143"
	caip2LayerX      = "eip155:196"
	caip2Adi         = "eip155:36900"
	caip2Base        = "eip155:8453"
	caip2Arbitrum    = "eip155:42161"
	caip2Avalanche   = "eip155:43114"
	caip2Berachain   = "eip155:80085"
	caip2Solana      = "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"
	caip2Tron        = "tron:27Lqcw"
	caip2XRPL        = "xrpl:0"
	caip2Sui         = "sui:mainnet"
	caip2Aptos       = "aptos:mainnet"
	caip2Cardano     = "cip34:1-764824073"
	caip2Starknet    = "starknet:SN_MAIN"
	caip2Plasma      = "eip155:9745"
	caip2Scroll      = "eip155:534352"
)

// ---------------------------------------------------------------------------------------------------
// internal-utils: request / retry / requestShouldRetry / handleRPCResponse / jsonRPCRequest
// ---------------------------------------------------------------------------------------------------

type retryOptions struct {
	delay       time.Duration
	minDelay    time.Duration
	factor      float64
	maxAttempts int
}

// RETRY_CONFIGS (utils/retry.js)
var retryThirtySecsAggressive = retryOptions{delay: 500 * time.Millisecond, minDelay: 500 * time.Millisecond, factor: 2, maxAttempts: 6}

const defaultRequestTimeout = 10 * time.Second

type httpRequestError struct {
	status  int
	details string
}

func (e httpRequestError) Error() string {
	return fmt.Sprintf("HTTP request failed. Status: %d Details: %s", e.status, e.details)
}

type rpcRequestError struct {
	details string
}

func (e rpcRequestError) Error() string {
	return "RPC Request failed. Details: " + e.details
}

func requestShouldRetry(err error) bool {
	var httpErr httpRequestError
	if errors.As(err, &httpErr) {
		switch httpErr.status {
		case 403, 408, 413, 429, 500, 502, 503, 504:
			return true
		}
		return false
	}
	return true
}

// calculateDelay of @lifeomic/attempt with jitter
func calculateRetryDelay(options retryOptions, attemptNum int) time.Duration {
	delay := float64(options.delay) * math.Pow(options.factor, float64(attemptNum-1))
	minDelay := int64(options.minDelay)
	maxDelay := int64(math.Floor(delay))
	return time.Duration(mathrand.Int63n(maxDelay-minDelay+1) + minDelay)
}

func requestOnce(ctx context.Context, method, requestURL string, body any, timeout time.Duration) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, method, requestURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpRequestError{status: resp.StatusCode, details: resp.Status}
	}
	return io.ReadAll(resp.Body)
}

func request(ctx context.Context, method, requestURL string, body any, timeout time.Duration, options retryOptions) ([]byte, error) {
	for attemptNum := 0; ; {
		response, err := requestOnce(ctx, method, requestURL, body, timeout)
		if err == nil {
			return response, nil
		}
		attemptNum++
		if ctx.Err() != nil || attemptNum >= options.maxAttempts || !requestShouldRetry(err) {
			return nil, err
		}
		if err := wait(ctx, calculateRetryDelay(options, attemptNum)); err != nil {
			return nil, err
		}
	}
}

func jsonRPCRequest(ctx context.Context, baseURL, method string, params any, timeout time.Duration, options retryOptions, out any, requestID ...string) error {
	rpcURL, err := url.JoinPath(baseURL, "rpc")
	if err != nil {
		return err
	}
	id := "dontcare"
	if len(requestID) > 0 {
		id = requestID[0]
	}
	body := map[string]any{
		"id":      id,
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		body["params"] = []any{params}
	}
	response, err := request(ctx, http.MethodPost, rpcURL+"?method="+url.QueryEscape(method), body, timeout, options)
	if err != nil {
		return err
	}

	var decoded struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(response, &decoded); err != nil {
		return rpcRequestError{details: "Invalid response"}
	}
	if len(decoded.Error) > 0 && string(decoded.Error) != "null" {
		var message string
		if json.Unmarshal(decoded.Error, &message) == nil {
			return rpcRequestError{details: message}
		}
		var structured struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(decoded.Error, &structured) == nil {
			return rpcRequestError{details: structured.Message}
		}
		return rpcRequestError{details: "Invalid response"}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(decoded.Result, out)
}

// ---------------------------------------------------------------------------------------------------
// internal-utils: poll (utils/poll.js)
// ---------------------------------------------------------------------------------------------------

type completionStats struct {
	p50 time.Duration
	p90 time.Duration
	p99 time.Duration
}

type pollTimeoutError struct {
	elapsed time.Duration
	timeout time.Duration
}

func (e pollTimeoutError) Error() string {
	return fmt.Sprintf("Polling timed out. Details: Operation did not complete within %dms (elapsed: %dms)", e.timeout.Milliseconds(), e.elapsed.Milliseconds())
}

func poll[T any](ctx context.Context, stats completionStats, fn func() (T, bool, error)) (T, error) {
	var zero T
	startTime := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		elapsed := time.Since(startTime)
		if elapsed >= stats.p99 {
			return zero, pollTimeoutError{elapsed: elapsed, timeout: stats.p99}
		}
		result, done, err := fn()
		if err != nil {
			return zero, err
		}
		if done {
			return result, nil
		}
		interval := 10 * time.Second
		if elapsed < stats.p50 {
			interval = 300 * time.Millisecond
		} else if elapsed < stats.p90 {
			interval = 3 * time.Second
		}
		if err := wait(ctx, interval); err != nil {
			return zero, err
		}
	}
}

// ---------------------------------------------------------------------------------------------------
// intents-sdk: nep413 (lib/nep413.js), expirable nonce, payload factory, IntentSignerNEP413, SaltManager
// ---------------------------------------------------------------------------------------------------

var versionedMagicPrefix = []byte{86, 40, 246, 198}

const latestNonceVersion = 0

func borshString(value string) []byte {
	out := make([]byte, 4, 4+len(value))
	binary.LittleEndian.PutUint32(out, uint32(len(value)))
	return append(out, value...)
}

func hashNEP413Message(message string, recipient string, nonce []byte) []byte {
	baseInt := make([]byte, 4)
	binary.LittleEndian.PutUint32(baseInt, uint32(1<<31+413))
	combinedData := append(baseInt, borshString(message)...)
	combinedData = append(combinedData, nonce...)
	combinedData = append(combinedData, borshString(recipient)...)
	combinedData = append(combinedData, 0) // callback_url: None
	hash := sha256.Sum256(combinedData)
	return hash[:]
}

func encodeNonce(salt []byte, deadline time.Time) (string, error) {
	if len(salt) != 4 {
		return "", fmt.Errorf("Invalid salt length: %d, expected 4", len(salt))
	}
	nonceBytes := make([]byte, 15)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	borshBytes := make([]byte, 27)
	copy(borshBytes, salt)
	binary.LittleEndian.PutUint64(borshBytes[4:], uint64(deadline.UnixMilli())*1_000_000)
	copy(borshBytes[12:], nonceBytes)
	result := make([]byte, 0, 5+len(borshBytes))
	result = append(result, versionedMagicPrefix...)
	result = append(result, latestNonceVersion)
	result = append(result, borshBytes...)
	return base64.StdEncoding.EncodeToString(result), nil
}

const defaultDeadline = 60 * time.Second

type intentPayload struct {
	VerifyingContract string
	Deadline          string
	Nonce             string
	Intents           []any
}

func defaultIntentPayloadFactory(salt []byte, intents []any, verifyingContract string) (intentPayload, error) {
	deadline := time.Now().Add(defaultDeadline)
	nonceDeadline := deadline.Add(defaultDeadline)
	nonce, err := encodeNonce(salt, nonceDeadline)
	if err != nil {
		return intentPayload{}, err
	}
	if intents == nil {
		intents = []any{}
	}
	return intentPayload{
		VerifyingContract: verifyingContract,
		Deadline:          deadline.UTC().Format("2006-01-02T15:04:05.000Z"),
		Nonce:             nonce,
		Intents:           intents,
	}, nil
}

type multiPayload struct {
	Standard string `json:"standard"`
	Payload  struct {
		Recipient string `json:"recipient"`
		Nonce     string `json:"nonce"`
		Message   string `json:"message"`
	} `json:"payload"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

type intentSignerNEP413 struct {
	accountID string
	publicKey string
	secretKey ed25519.PrivateKey
}

func marshalJSONNoEscape(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

func (s *intentSignerNEP413) signIntent(intent intentPayload) (multiPayload, error) {
	nonce, err := base64.StdEncoding.DecodeString(intent.Nonce)
	if err != nil {
		return multiPayload{}, err
	}
	message, err := marshalJSONNoEscape(struct {
		Deadline string `json:"deadline"`
		Intents  []any  `json:"intents"`
		SignerID string `json:"signer_id"`
	}{Deadline: intent.Deadline, Intents: intent.Intents, SignerID: s.accountID})
	if err != nil {
		return multiPayload{}, err
	}
	signature := ed25519.Sign(s.secretKey, hashNEP413Message(message, intent.VerifyingContract, nonce))

	var signed multiPayload
	signed.Standard = "nep413"
	signed.Payload.Recipient = intent.VerifyingContract
	signed.Payload.Nonce = intent.Nonce
	signed.Payload.Message = message
	signed.PublicKey = s.publicKey
	signed.Signature = "ed25519:" + base58.Encode(signature)
	return signed, nil
}

const saltTTL = 300 * time.Second

type saltManager struct {
	mu            sync.Mutex
	currentSalt   []byte
	lastFetchTime time.Time
}

func fetchSalt(ctx context.Context) ([]byte, error) {
	value, err := callFunction[string](ctx, constants.NearIntentsContractID, "current_salt", struct{}{})
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("empty salt")
	}
	return hex.DecodeString(*value)
}

func (m *saltManager) getCachedSalt(ctx context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentSalt != nil && time.Since(m.lastFetchTime) < saltTTL {
		return m.currentSalt, nil
	}
	salt, err := fetchSalt(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch salt: %w", err)
	}
	m.currentSalt = salt
	m.lastFetchTime = time.Now()
	return salt, nil
}

func (m *saltManager) refresh(ctx context.Context) ([]byte, error) {
	m.mu.Lock()
	m.currentSalt = nil
	m.lastFetchTime = time.Time{}
	m.mu.Unlock()
	return m.getCachedSalt(ctx)
}

// ---------------------------------------------------------------------------------------------------
// internal-utils: solverRelay (publishIntents, getStatus, waitForIntentSettlement, getQuote)
// ---------------------------------------------------------------------------------------------------

type relayPublishError struct {
	reason string
	code   string
}

func (e relayPublishError) Error() string {
	return "Failed to publish intent. Details: " + e.reason + " Code: " + e.code
}

func parseFailedPublishReason(reason string) string {
	switch {
	case reason == "expired" || strings.Contains(reason, "deadline has expired"):
		return "SIGNATURE_EXPIRED"
	case reason == "internal":
		return "INTERNAL_ERROR"
	case strings.Contains(reason, "invalid signature"):
		return "SIGNATURE_INVALID"
	case strings.Contains(reason, "nonce was already used"):
		return "NONCE_USED"
	case strings.Contains(reason, "insufficient balance or overflow"):
		return "INSUFFICIENT_BALANCE"
	case strings.Contains(reason, "public key doesn't exist"):
		return "PUBLIC_KEY_NOT_EXIST"
	case strings.Contains(reason, "invalid salt"):
		return "INVALID_SALT"
	default:
		return "UNKNOWN_ERROR"
	}
}

func publishIntents(ctx context.Context, multiPayloads []multiPayload, quoteHashes []string) ([]string, error) {
	if quoteHashes == nil {
		quoteHashes = []string{}
	}
	var response struct {
		Status       string   `json:"status"`
		Reason       string   `json:"reason"`
		IntentHashes []string `json:"intent_hashes"`
	}
	if err := jsonRPCRequest(ctx, envSolverRelayBaseURL, "publish_intents", map[string]any{
		"quote_hashes": quoteHashes,
		"signed_datas": multiPayloads,
	}, 30*time.Second, retryOptions{delay: time.Second, minDelay: time.Second, factor: 1.5, maxAttempts: 7}, &response); err != nil {
		return nil, relayPublishError{reason: "Error occurred during sending a request", code: "NETWORK_ERROR"}
	}
	if response.Status == "OK" || response.Reason == "already processed" {
		return response.IntentHashes, nil
	}
	return nil, relayPublishError{reason: response.Reason, code: parseFailedPublishReason(response.Reason)}
}

// DEFAULT_SETTLEMENT_STATS (solverRelay/waitForIntentSettlement.js)
var defaultSettlementStats = completionStats{p50: 2 * time.Second, p90: 10 * time.Second, p99: 356 * time.Second}

func waitForIntentSettlement(ctx context.Context, intentHash string) (string, error) {
	return poll(ctx, defaultSettlementStats, func() (string, bool, error) {
		var response struct {
			IntentHash    string `json:"intent_hash"`
			Status        string `json:"status"`
			StatusDetails string `json:"status_details"`
			Data          *struct {
				Hash string `json:"hash"`
			} `json:"data"`
		}
		err := jsonRPCRequest(ctx, envSolverRelayBaseURL, "get_status", map[string]any{"intent_hash": intentHash}, defaultRequestTimeout, retryThirtySecsAggressive, &response)
		if err != nil {
			if ctx.Err() != nil {
				return "", false, ctx.Err()
			}
			// isTransientError
			return "", false, nil
		}
		switch response.Status {
		case "SETTLED":
			if response.Data == nil {
				return "", false, fmt.Errorf("Intent settled without tx hash. Intent hash: %s", response.IntentHash)
			}
			return response.Data.Hash, true, nil
		case "TX_BROADCASTED", "PENDING":
			return "", false, nil
		case "NOT_FOUND_OR_NOT_VALID":
			if response.StatusDetails == "FAILED" {
				return "", false, fmt.Errorf("Intent settlement failed. Details: Intent is published but not settled. Status: %s Intent hash: %s", response.Status, response.IntentHash)
			}
			return "", false, nil
		default:
			// raw getStatus (solverRelayHttpClient/apis.js) does not assert on unknown statuses, so keep polling
			return "", false, nil
		}
	})
}

type relayQuote struct {
	QuoteHash                string `json:"quote_hash"`
	DefuseAssetIdentifierIn  string `json:"defuse_asset_identifier_in"`
	DefuseAssetIdentifierOut string `json:"defuse_asset_identifier_out"`
	AmountIn                 string `json:"amount_in"`
	AmountOut                string `json:"amount_out"`
	Type                     string `json:"type,omitempty"`
}

type quoteError struct {
	details string
}

func (e quoteError) Error() string { return "Quote error. Details: " + e.details }

func getQuote(ctx context.Context, quoteParams map[string]string) (relayQuote, error) {
	var result []relayQuote
	if err := jsonRPCRequest(ctx, envSolverRelayBaseURL, "quote", quoteParams, defaultRequestTimeout, retryThirtySecsAggressive, &result, uuid.NewString()); err != nil {
		return relayQuote{}, err
	}
	if result == nil {
		return relayQuote{}, quoteError{details: "NO_QUOTE"}
	}
	validQuotes := make([]relayQuote, 0, len(result))
	failedQuotes := make([]relayQuote, 0, len(result))
	for _, q := range result {
		if q.Type == "" {
			validQuotes = append(validQuotes, q)
		} else {
			failedQuotes = append(failedQuotes, q)
		}
	}
	// sortQuotes checks `exact_amount_in !== null`, which is true when the field is absent,
	// so quotes are always sorted by amount_out descending
	slices.SortStableFunc(validQuotes, func(a, b relayQuote) int {
		return mustBigInt(b.AmountOut).Cmp(mustBigInt(a.AmountOut))
	})
	if len(validQuotes) == 0 {
		if len(failedQuotes) > 0 {
			return relayQuote{}, quoteError{details: failedQuotes[0].Type}
		}
		return relayQuote{}, quoteError{details: "NO_QUOTE"}
	}
	return validQuotes[0], nil
}

func mustBigInt(value string) *big.Int {
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return big.NewInt(0)
	}
	return parsed
}

type tokenUsdPrice struct {
	DefuseAssetID string  `json:"defuse_asset_id"`
	Price         float64 `json:"price"`
	Decimals      int     `json:"decimals"`
}

// getFeeQuote (lib/estimate-fee.js): ExactIn fallback with 1.2x multiplier
func getFeeQuote(ctx context.Context, feeAmount *big.Int, feeAssetID string, tokenAssetID string) (relayQuote, error) {
	quote, err := getQuote(ctx, map[string]string{
		"defuse_asset_identifier_in":  tokenAssetID,
		"defuse_asset_identifier_out": feeAssetID,
		"exact_amount_out":            feeAmount.String(),
	})
	var qErr quoteError
	if err == nil || !errors.As(err, &qErr) {
		return quote, err
	}

	pricesURL, joinErr := url.JoinPath(envManagerConsoleBaseURL, "tokens")
	if joinErr != nil {
		return relayQuote{}, joinErr
	}
	response, requestErr := request(ctx, http.MethodGet, pricesURL, nil, defaultRequestTimeout, retryOptions{maxAttempts: 1})
	if requestErr != nil {
		return relayQuote{}, requestErr
	}
	var prices struct {
		Items []tokenUsdPrice `json:"items"`
	}
	if err := json.Unmarshal(response, &prices); err != nil {
		return relayQuote{}, err
	}
	feeIndex := slices.IndexFunc(prices.Items, func(t tokenUsdPrice) bool { return t.DefuseAssetID == feeAssetID })
	tokenIndex := slices.IndexFunc(prices.Items, func(t tokenUsdPrice) bool { return t.DefuseAssetID == tokenAssetID })
	if feeIndex < 0 || tokenIndex < 0 {
		return relayQuote{}, err
	}
	feeAssetPrice, tokenAssetPrice := prices.Items[feeIndex], prices.Items[tokenIndex]

	const usdScale = 1e6
	feePriceScaled := big.NewInt(int64(math.Round(feeAssetPrice.Price * usdScale)))
	tokenPriceScaled := big.NewInt(int64(math.Round(tokenAssetPrice.Price * usdScale)))
	num := new(big.Int).Mul(feeAmount, feePriceScaled)
	num.Mul(num, big.NewInt(12))
	num.Mul(num, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenAssetPrice.Decimals)), nil))
	den := new(big.Int).Mul(tokenPriceScaled, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(feeAssetPrice.Decimals)), nil))
	den.Mul(den, big.NewInt(10))
	if den.Sign() == 0 { // a token price that rounds to 0 at USD_SCALE; JS BigInt division throws RangeError here
		return relayQuote{}, errors.New("Division by zero")
	}
	exactAmountIn, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	if remainder.Sign() != 0 {
		exactAmountIn.Add(exactAmountIn, big.NewInt(1))
	}
	if exactAmountIn.Sign() == 0 {
		exactAmountIn = big.NewInt(1)
	}

	quote, quoteErr := getQuote(ctx, map[string]string{
		"defuse_asset_identifier_in":  tokenAssetID,
		"defuse_asset_identifier_out": feeAssetID,
		"exact_amount_in":             exactAmountIn.String(),
	})
	if quoteErr != nil {
		return relayQuote{}, quoteErr
	}
	actualRatio := new(big.Int).Div(new(big.Int).Mul(mustBigInt(quote.AmountOut), big.NewInt(1000)), feeAmount)
	if actualRatio.Cmp(big.NewInt(1500)) > 0 {
		return relayQuote{}, err
	}
	if mustBigInt(quote.AmountOut).Cmp(feeAmount) < 0 {
		return relayQuote{}, err
	}
	return quote, nil
}

// ---------------------------------------------------------------------------------------------------
// intents-sdk: shared bridge types, validateAddress, withdrawal timing
// ---------------------------------------------------------------------------------------------------

type withdrawalParams struct {
	AssetID            string
	Amount             *big.Int
	DestinationAddress string
	FeeInclusive       bool
}

type feeEstimation struct {
	amount         *big.Int
	quote          *relayQuote
	underlyingFees map[string]map[string]*big.Int
}

func getUnderlyingFee(fee feeEstimation, route string, feeKey string) (*big.Int, error) {
	routeFees, ok := fee.underlyingFees[route]
	if !ok {
		return nil, fmt.Errorf("Missing underlying fees for route %q. Fee estimation must populate underlyingFees before creating withdrawal intents.", route)
	}
	return routeFees[feeKey], nil
}

type withdrawalStatus struct {
	status string
	reason string
}

type withdrawalIdentifier struct {
	landingChain     string
	index            int
	withdrawalParams withdrawalParams
	txHash           string
}

type withdrawalBridge interface {
	route() string
	supports(ctx context.Context, params withdrawalParams) (bool, error)
	estimateWithdrawalFee(ctx context.Context, params withdrawalParams) (feeEstimation, error)
	validateWithdrawal(ctx context.Context, assetID string, amount *big.Int, destinationAddress string, fee feeEstimation) error
	createWithdrawalIntents(params withdrawalParams, fee feeEstimation, referral string) ([]any, error)
	createWithdrawalIdentifier(params withdrawalParams, index int, txHash string) (withdrawalIdentifier, error)
	describeWithdrawal(ctx context.Context, wid withdrawalIdentifier) (withdrawalStatus, error)
}

type parsedDefuseAssetID struct {
	standard   string
	contractID string
	tokenID    string
}

var nearAccountIDRegex = regexp.MustCompile(`^(([a-z\d]+[-_])*[a-z\d]+\.)*([a-z\d]+[-_])*[a-z\d]+$`)

var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func validateNearAddress(accountID string) bool {
	if len(accountID) < 2 || len(accountID) > 64 {
		return false
	}
	if isImplicitAccount(accountID) {
		return true
	}
	return nearAccountIDRegex.MatchString(accountID)
}

func isImplicitAccount(accountID string) bool {
	isEthImplicit := len(accountID) == 42 && strings.HasPrefix(accountID, "0x") && hexPattern.MatchString(accountID[2:])
	isNearImplicit := len(accountID) == 64 && hexPattern.MatchString(accountID)
	isNearDeterministic := len(accountID) == 42 && strings.HasPrefix(accountID, "0s") && hexPattern.MatchString(accountID[2:])
	return isEthImplicit || isNearImplicit || isNearDeterministic
}

func parseDefuseAssetID(assetID string) (parsedDefuseAssetID, error) {
	parts := strings.Split(assetID, ":")
	invalid := fmt.Errorf("Invalid asset id format. Asset ID: %s", assetID)
	if len(parts) < 2 || !validateNearAddress(parts[1]) {
		return parsedDefuseAssetID{}, invalid
	}
	switch parts[0] {
	case "nep141":
		return parsedDefuseAssetID{standard: "nep141", contractID: parts[1]}, nil
	case "nep245":
		if len(parts) < 3 {
			return parsedDefuseAssetID{}, invalid
		}
		return parsedDefuseAssetID{standard: "nep245", contractID: parts[1], tokenID: parts[2]}, nil
	default:
		return parsedDefuseAssetID{}, invalid
	}
}

var ethAddressRegex = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)

var evmCaip2Chains = []string{
	caip2Ethereum, caip2Optimism, caip2BNB, caip2Gnosis, caip2Polygon, caip2Monad, caip2LayerX, caip2Adi,
	caip2Base, caip2Arbitrum, caip2Avalanche, caip2Berachain, caip2Plasma, caip2Scroll,
}

func validateTronBase58Address(address string) bool {
	decoded, err := base58.Decode(address)
	if err != nil || len(decoded) != 25 {
		return false
	}
	first := sha256.Sum256(decoded[:21])
	second := sha256.Sum256(first[:])
	return bytes.Equal(decoded[21:], second[:4]) && decoded[0] == 65
}

func validateTronHexAddress(address string) bool {
	decoded, err := hex.DecodeString(address)
	return err == nil && len(decoded) == 21 && decoded[0] == 65
}

func validateEthAddress(address string) bool {
	if !ethAddressRegex.MatchString(address) {
		return false
	}
	if strings.ToLower(address) == address {
		return true
	}
	return common.HexToAddress(address).Hex() == address
}

// validateAddress (lib/validateAddress.js): Solana, Tron and EVM branches
func validateAddress(address string, blockchain string) bool {
	switch {
	case blockchain == caip2Solana:
		publicKey, err := solana.PublicKeyFromBase58(address)
		return err == nil && publicKey.IsOnCurve()
	case blockchain == caip2Tron:
		return validateTronBase58Address(address) || validateTronHexAddress(address)
	case slices.Contains(evmCaip2Chains, blockchain):
		return validateEthAddress(address)
	default:
		return false
	}
}

// WITHDRAWAL_P99_BY_CHAIN (constants/withdrawal-timing.js)
var withdrawalP99ByChain = map[string]int64{
	"eip155:1": 1852, "eip155:56": 36, "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp": 1387, "eip155:8453": 651,
	"tron:27Lqcw": 2358, "eip155:42161": 928, "bip122:000000000019d6689c085ae165831e93": 3656, "eip155:137": 410,
	"xrpl:0": 2276, "bip122:00040fe8ec8471911baa1db1266ea15d": 2093, "tvm:-239": 53, "near:mainnet": 356,
	"bip122:12a765e31ffd4059bada1e25190f6e98": 2385, "eip155:143": 22, "eip155:43114": 657, "stellar:pubnet": 50,
	"eip155:10": 23, "bip122:1a91e3dace36e2be3bf030a65679fe82": 1970, "sui:mainnet": 752, "eip155:80085": 579,
	"eip155:100": 3308, "aptos:mainnet": 394, "cip34:1-764824073": 807,
}

// HOT_EXTRA_TIMEOUT_MS (constants/withdrawal-timing.js)
const hotExtraTimeout = 720 * time.Second

// getWithdrawalStatsForChain: the timeout (p99 field) is 3x the chain p99, plus 12 minutes on the HOT bridge.
func getWithdrawalStatsForChain(chain string, bridgeRoute string) completionStats {
	var extraTimeout time.Duration
	if bridgeRoute == routeHotBridge {
		extraTimeout = hotExtraTimeout
	}
	p99Seconds, ok := withdrawalP99ByChain[chain]
	if !ok {
		return completionStats{p50: 60 * time.Second, p90: 600 * time.Second, p99: 21600*time.Second + extraTimeout}
	}
	p99 := time.Duration(p99Seconds) * time.Second
	return completionStats{
		p50: max(5*time.Second, time.Duration(float64(p99)*0.15)),
		p90: max(30*time.Second, time.Duration(float64(p99)*0.5)),
		p99: p99*3 + extraTimeout,
	}
}

// getNearNep141MinStorageBalance / getNearNep141StorageBalance (internal-utils services/blockchainBalanceService.js)
func getNearNep141MinStorageBalance(ctx context.Context, contractID string) (*big.Int, error) {
	bounds, err := callFunction[struct {
		Min string `json:"min"`
	}](ctx, contractID, "storage_balance_bounds", struct{}{})
	if err != nil {
		return nil, err
	}
	if bounds == nil {
		return nil, errors.New("storage_balance_bounds returned no value")
	}
	return mustBigInt(bounds.Min), nil
}

func getNearNep141StorageBalance(ctx context.Context, contractID string, accountID string) (*big.Int, error) {
	balance, err := callFunction[*struct {
		Total string `json:"total"`
	}](ctx, contractID, "storage_balance_of", map[string]string{"account_id": accountID})
	if err != nil {
		return nil, fmt.Errorf("Error fetching balance: %w", err)
	}
	if balance == nil || *balance == nil || (*balance).Total == "" {
		return big.NewInt(0), nil
	}
	return mustBigInt((*balance).Total), nil
}

// Promise.all([getNearNep141MinStorageBalance, getNearNep141StorageBalance])
func getNearNep141StorageBalances(ctx context.Context, contractID string, accountID string) ([2]*big.Int, error) {
	var result [2]*big.Int
	var g errgroup.Group
	g.Go(func() (err error) {
		result[0], err = getNearNep141MinStorageBalance(ctx, contractID)
		return err
	})
	g.Go(func() (err error) {
		result[1], err = getNearNep141StorageBalance(ctx, contractID, accountID)
		return err
	})
	if err := g.Wait(); err != nil {
		return [2]*big.Int{}, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------------------------------
// intents-sdk: PoaBridge (bridges/poa-bridge)
// ---------------------------------------------------------------------------------------------------

const poaMinGasAmount = "17050000000000"

var poaTokenPrefixMapping = []struct {
	prefix string
	caip2  string
}{
	{"eth", caip2Ethereum}, {"base", caip2Base}, {"arb", caip2Arbitrum}, {"btc", caip2Bitcoin},
	{"bch", caip2BitcoinCash}, {"sol", caip2Solana}, {"doge", caip2Dogecoin}, {"xrp", caip2XRPL},
	{"zec", caip2Zcash}, {"gnosis", caip2Gnosis}, {"bera", caip2Berachain}, {"tron", caip2Tron},
	{"sui", caip2Sui}, {"aptos", caip2Aptos}, {"cardano", caip2Cardano}, {"ltc", caip2Litecoin},
	{"starknet", caip2Starknet},
}

var poaCaip2Mapping = map[string]string{
	caip2Ethereum: "eth:1", caip2Base: "eth:8453", caip2Arbitrum: "eth:42161", caip2Bitcoin: "btc:mainnet",
	caip2BitcoinCash: "bch:mainnet", caip2Solana: "sol:mainnet", caip2Dogecoin: "doge:mainnet",
	caip2XRPL: "xrp:mainnet", caip2Zcash: "zec:mainnet", caip2Gnosis: "eth:100", caip2Berachain: "eth:80094",
	caip2Tron: "tron:mainnet", caip2Sui: "sui:mainnet", caip2Aptos: "aptos:mainnet",
	caip2Cardano: "cardano:mainnet", caip2Litecoin: "ltc:mainnet", caip2Starknet: "starknet:mainnet",
}

func toPoaNetwork(caip2 string) (string, error) {
	network, ok := poaCaip2Mapping[caip2]
	if !ok {
		return "", fmt.Errorf("Unsupported POA Bridge chain = %s", caip2)
	}
	return network, nil
}

func poaContractIDToCaip2(contractID string) (string, error) {
	for _, mapping := range poaTokenPrefixMapping {
		if strings.HasPrefix(contractID, mapping.prefix+".") || strings.HasPrefix(contractID, mapping.prefix+"-") {
			return mapping.caip2, nil
		}
	}
	return "", fmt.Errorf("Unsupported POA Bridge contractId = %s", contractID)
}

type poaSupportedTokens struct {
	Tokens []struct {
		IntentsTokenID      string `json:"intents_token_id"`
		MinWithdrawalAmount string `json:"min_withdrawal_amount"`
	} `json:"tokens"`
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

type ttlCache[T any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]cacheEntry[T]
}

func newTTLCache[T any](ttl time.Duration) *ttlCache[T] {
	return &ttlCache[T]{ttl: ttl, entries: map[string]cacheEntry[T]{}}
}

func (c *ttlCache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		var zero T
		return zero, false
	}
	return entry.value, true
}

func (c *ttlCache[T]) set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry[T]{value: value, expiresAt: time.Now().Add(c.ttl)}
}

type poaBridge struct {
	supportedTokensCache *ttlCache[poaSupportedTokens]
}

func newPoaBridge() *poaBridge {
	return &poaBridge{supportedTokensCache: newTTLCache[poaSupportedTokens](30 * time.Second)}
}

func (b *poaBridge) route() string { return routePoaBridge }

func (b *poaBridge) parseAssetID(assetID string) (parsedDefuseAssetID, string, bool, error) {
	parsed, err := parseDefuseAssetID(assetID)
	if err != nil {
		return parsedDefuseAssetID{}, "", false, err
	}
	if !strings.HasSuffix(parsed.contractID, "."+envPoaTokenFactoryContractID) {
		return parsedDefuseAssetID{}, "", false, nil
	}
	blockchain, err := poaContractIDToCaip2(parsed.contractID)
	if err != nil {
		return parsedDefuseAssetID{}, "", false, fmt.Errorf("Asset belongs to unknown blockchain. Asset ID: %s", assetID)
	}
	return parsed, blockchain, true, nil
}

func (b *poaBridge) supports(_ context.Context, params withdrawalParams) (bool, error) {
	_, _, ok, err := b.parseAssetID(params.AssetID)
	return ok, err
}

func (b *poaBridge) createWithdrawalIntents(params withdrawalParams, fee feeEstimation, _ string) ([]any, error) {
	relayerFee, err := getUnderlyingFee(fee, routePoaBridge, "relayerFee")
	if err != nil {
		return nil, err
	}
	if relayerFee == nil || relayerFee.Sign() <= 0 {
		return nil, fmt.Errorf("Invalid POA bridge relayer fee: expected > 0, got %s", relayerFee)
	}
	parsed, err := parseDefuseAssetID(params.AssetID)
	if err != nil {
		return nil, err
	}
	return []any{struct {
		Intent     string `json:"intent"`
		Token      string `json:"token"`
		ReceiverID string `json:"receiver_id"`
		Amount     string `json:"amount"`
		Memo       string `json:"memo"`
		MinGas     string `json:"min_gas"`
	}{
		Intent:     "ft_withdraw",
		Token:      parsed.contractID,
		ReceiverID: parsed.contractID,
		Amount:     new(big.Int).Add(params.Amount, relayerFee).String(),
		Memo:       createWithdrawMemo(params.DestinationAddress),
		MinGas:     poaMinGasAmount,
	}}, nil
}

func createWithdrawMemo(receiverAddress string) string {
	if strings.HasPrefix(strings.ToLower(receiverAddress), "bitcoincash:") {
		receiverAddress = receiverAddress[12:]
	}
	return strings.Join([]string{"WITHDRAW_TO", receiverAddress}, ":")
}

func (b *poaBridge) validateWithdrawal(ctx context.Context, assetID string, amount *big.Int, destinationAddress string, _ feeEstimation) error {
	_, blockchain, ok, err := b.parseAssetID(assetID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("Asset is not supported")
	}
	if !validateAddress(destinationAddress, blockchain) {
		return fmt.Errorf("Invalid destination address. Details: Destination address %s is not valid for withdrawal to %s.", destinationAddress, blockchain)
	}
	network, err := toPoaNetwork(blockchain)
	if err != nil {
		return err
	}
	supportedTokens, err := b.getCachedSupportedTokens(ctx, []string{network})
	if err != nil {
		return err
	}
	for _, token := range supportedTokens.Tokens {
		if token.IntentsTokenID != assetID {
			continue
		}
		minWithdrawalAmount := mustBigInt(token.MinWithdrawalAmount)
		if amount.Cmp(minWithdrawalAmount) < 0 {
			return fmt.Errorf("Withdrawal amount is below minimum required by the bridge. Asset ID: %s Minimum amount: %s Requested amount: %s", assetID, minWithdrawalAmount, amount)
		}
		break
	}
	return nil
}

func (b *poaBridge) estimateWithdrawalFee(ctx context.Context, params withdrawalParams) (feeEstimation, error) {
	parsed, blockchain, ok, err := b.parseAssetID(params.AssetID)
	if err != nil {
		return feeEstimation{}, err
	}
	if !ok {
		return feeEstimation{}, errors.New("Asset is not supported")
	}
	network, err := toPoaNetwork(blockchain)
	if err != nil {
		return feeEstimation{}, err
	}
	var estimation struct {
		WithdrawalFee string `json:"withdrawalFee"`
	}
	if err := jsonRPCRequest(ctx, envPoaBridgeBaseURL, "withdrawal_estimate", map[string]string{
		"token":   parsed.contractID,
		"address": params.DestinationAddress,
		"chain":   network,
	}, defaultRequestTimeout, retryThirtySecsAggressive, &estimation); err != nil {
		return feeEstimation{}, err
	}
	relayerFee := mustBigInt(estimation.WithdrawalFee)
	if relayerFee.Sign() <= 0 {
		return feeEstimation{}, fmt.Errorf("Invalid POA bridge relayer fee: expected > 0, got %s", relayerFee)
	}
	return feeEstimation{
		amount:         relayerFee,
		underlyingFees: map[string]map[string]*big.Int{routePoaBridge: {"relayerFee": relayerFee}},
	}, nil
}

func (b *poaBridge) createWithdrawalIdentifier(params withdrawalParams, index int, txHash string) (withdrawalIdentifier, error) {
	_, blockchain, ok, err := b.parseAssetID(params.AssetID)
	if err != nil {
		return withdrawalIdentifier{}, err
	}
	if !ok {
		return withdrawalIdentifier{}, errors.New("Asset is not supported")
	}
	return withdrawalIdentifier{landingChain: blockchain, index: index, withdrawalParams: params, txHash: txHash}, nil
}

func (b *poaBridge) describeWithdrawal(ctx context.Context, wid withdrawalIdentifier) (withdrawalStatus, error) {
	status, err := b.getWithdrawalStatusWithRetry(ctx, wid.txHash)
	if err != nil {
		return withdrawalStatus{}, err
	}
	for _, withdrawal := range status.Withdrawals {
		if "nep141:"+withdrawal.Data.NearTokenID != wid.withdrawalParams.AssetID {
			continue
		}
		switch withdrawal.Status {
		case "PENDING":
			return withdrawalStatus{status: "pending"}, nil
		case "COMPLETED":
			return withdrawalStatus{status: "completed"}, nil
		default:
			return withdrawalStatus{status: "failed", reason: withdrawal.Status}, nil
		}
	}
	return withdrawalStatus{status: "pending"}, nil
}

type poaWithdrawalStatusResponse struct {
	Withdrawals []struct {
		Status string `json:"status"`
		Data   struct {
			NearTokenID string `json:"near_token_id"`
		} `json:"data"`
	} `json:"withdrawals"`
}

func (b *poaBridge) getWithdrawalStatusWithRetry(ctx context.Context, withdrawalHash string) (poaWithdrawalStatusResponse, error) {
	startTime := time.Now()
	for {
		var status poaWithdrawalStatusResponse
		err := jsonRPCRequest(ctx, envPoaBridgeBaseURL, "withdrawal_status", map[string]string{"withdrawal_hash": withdrawalHash}, defaultRequestTimeout, retryThirtySecsAggressive, &status)
		if err == nil {
			return status, nil
		}
		var rpcErr rpcRequestError
		if !errors.As(err, &rpcErr) || rpcErr.details != "Withdrawals not found" {
			return poaWithdrawalStatusResponse{}, err
		}
		if time.Since(startTime) >= 3*time.Second {
			return poaWithdrawalStatusResponse{}, nil
		}
		if err := wait(ctx, time.Second); err != nil {
			return poaWithdrawalStatusResponse{}, err
		}
	}
}

func (b *poaBridge) getCachedSupportedTokens(ctx context.Context, chains []string) (poaSupportedTokens, error) {
	sorted := slices.Clone(chains)
	slices.Sort(sorted)
	cacheKey := strings.Join(sorted, ",")
	if cached, ok := b.supportedTokensCache.get(cacheKey); ok {
		return cached, nil
	}
	var data poaSupportedTokens
	if err := jsonRPCRequest(ctx, envPoaBridgeBaseURL, "supported_tokens", map[string][]string{"chains": chains}, defaultRequestTimeout, retryThirtySecsAggressive, &data); err != nil {
		return poaSupportedTokens{}, err
	}
	b.supportedTokensCache.set(cacheKey, data)
	return data, nil
}

// ---------------------------------------------------------------------------------------------------
// intents-sdk: OmniBridge (bridges/omni-bridge) with the omni-bridge-sdk helpers it uses
// ---------------------------------------------------------------------------------------------------

const (
	nearNativeAssetID                      = "nep141:wrap.near"
	omniBridgeContract                     = "omni.bridge.near"
	omniMinGasAmount                       = "37400000000000"
	omniBridgeAPIBaseURL                   = "https://mainnet.api.bridge.nearone.org"
	omniWithdrawalAPIFeeRequestTimeout     = 3 * time.Second
	minAllowedStorageBalanceForIntentsNear = "500000000000000000000000"
)

// ChainKind (omni-bridge-sdk types/chain.js)
type omniChainKind int

const (
	chainKindEth omniChainKind = iota
	chainKindNear
	chainKindSol
	chainKindArb
	chainKindBase
	chainKindBnb
	chainKindBtc
	chainKindZcash
)

var chainKindNames = map[omniChainKind]string{
	chainKindEth: "Eth", chainKindNear: "Near", chainKindSol: "Sol", chainKindArb: "Arb",
	chainKindBase: "Base", chainKindBnb: "Bnb", chainKindBtc: "Btc", chainKindZcash: "Zcash",
}

// CHAIN_MAPPINGS (omni-bridge-utils.js)
var omniChainMappings = []struct {
	caip2 string
	kind  omniChainKind
}{
	{caip2Ethereum, chainKindEth}, {caip2Base, chainKindBase}, {caip2Arbitrum, chainKindArb},
	{caip2Solana, chainKindSol}, {caip2BNB, chainKindBnb}, {caip2Bitcoin, chainKindBtc},
}

func caip2ToChainKind(network string) (omniChainKind, bool) {
	for _, mapping := range omniChainMappings {
		if mapping.caip2 == network {
			return mapping.kind, true
		}
	}
	return 0, false
}

func chainKindToCaip2(kind omniChainKind) (string, bool) {
	for _, mapping := range omniChainMappings {
		if mapping.kind == kind {
			return mapping.caip2, true
		}
	}
	return "", false
}

func isUtxoChain(kind omniChainKind) bool {
	return kind == chainKindBtc
}

func omniAddress(chain omniChainKind, address string) string {
	return strings.ToLower(chainKindNames[chain]) + ":" + address
}

func getOmniChain(address string) (omniChainKind, bool) {
	prefix := strings.Split(address, ":")[0]
	kind, ok := map[string]omniChainKind{
		"eth": chainKindEth, "near": chainKindNear, "sol": chainKindSol, "arb": chainKindArb,
		"base": chainKindBase, "bnb": chainKindBnb, "btc": chainKindBtc, "zec": chainKindZcash,
	}[prefix]
	return kind, ok
}

// CHAIN_PATTERNS (omni-bridge-sdk utils/tokens.js)
var omniChainPatterns = map[string]omniChainKind{
	"nbtc.bridge.near": chainKindBtc, "eth.bridge.near": chainKindEth, "sol.omdep.near": chainKindSol,
	"base.omdep.near": chainKindBase, "arb.omdep.near": chainKindArb, "bnb.omdep.near": chainKindBnb,
	"nbtc.n-bridge.testnet": chainKindBtc, "eth.sepolia.testnet": chainKindEth, "sol.omnidep.testnet": chainKindSol,
	"base.omnidep.testnet": chainKindBase, "arb.omnidep.testnet": chainKindArb, "bnb.omnidep.testnet": chainKindBnb,
}

var omniBridgeTokenSuffix = regexp.MustCompile(`\.(omdep\.near|omnidep\.testnet|factory\.bridge\.(near|testnet))$`)

func isBridgeToken(nearAddress string) bool {
	_, ok := omniChainPatterns[nearAddress]
	return ok || omniBridgeTokenSuffix.MatchString(nearAddress)
}

func parseOriginChain(nearAddress string) (omniChainKind, bool) {
	if kind, ok := omniChainPatterns[nearAddress]; ok {
		return kind, true
	}
	if omniBridgeTokenSuffix.MatchString(nearAddress) {
		switch {
		case strings.HasPrefix(nearAddress, "sol-"):
			return chainKindSol, true
		case strings.HasPrefix(nearAddress, "base-"):
			return chainKindBase, true
		case strings.HasPrefix(nearAddress, "arb-"):
			return chainKindArb, true
		case strings.HasPrefix(nearAddress, "bnb-"):
			return chainKindBnb, true
		case strings.Contains(nearAddress, "factory.bridge"):
			return chainKindEth, true
		}
	}
	return 0, false
}

func validateOmniToken(nearAddress string) bool {
	if strings.HasSuffix(nearAddress, ".testnet") {
		return false
	}
	return isBridgeToken(nearAddress)
}

// calculateStorageAccountId (omni-bridge-sdk utils/storage.js)
func borshOmniAddress(address string) ([]byte, error) {
	parts := strings.SplitN(address, ":", 2)
	if len(parts) < 2 || parts[1] == "" {
		return nil, fmt.Errorf("Invalid token address format: %s", address)
	}
	decodeHex := func(value string) ([]byte, error) { return hex.DecodeString(value[2:]) }
	switch parts[0] {
	case "eth", "arb", "base", "bnb":
		decoded, err := decodeHex(parts[1])
		if err != nil {
			return nil, err
		}
		variant := map[string]byte{"eth": 0, "arb": 3, "base": 4, "bnb": 5}[parts[0]]
		return append([]byte{variant}, decoded...), nil
	case "near":
		return append([]byte{1}, borshString(parts[1])...), nil
	case "sol":
		decoded, err := base58.Decode(parts[1])
		if err != nil {
			return nil, err
		}
		return append([]byte{2}, decoded...), nil
	case "btc":
		return append([]byte{6}, borshString(parts[1])...), nil
	case "zcash":
		return append([]byte{7}, borshString(parts[1])...), nil
	default:
		return nil, fmt.Errorf("Unknown chain: %s", parts[0])
	}
}

func borshU128(value *big.Int) []byte {
	out := make([]byte, 16)
	bigEndian := value.FillBytes(make([]byte, 16))
	for i := range out {
		out[i] = bigEndian[15-i]
	}
	return out
}

func calculateStorageAccountID(token string, amount *big.Int, recipient string, fee *big.Int, nativeFee *big.Int, sender string, msg string) (string, error) {
	tokenBytes, err := borshOmniAddress(token)
	if err != nil {
		return "", err
	}
	recipientBytes, err := borshOmniAddress(recipient)
	if err != nil {
		return "", err
	}
	senderBytes, err := borshOmniAddress(sender)
	if err != nil {
		return "", err
	}
	serialized := append([]byte{}, tokenBytes...)
	serialized = append(serialized, borshU128(amount)...)
	serialized = append(serialized, recipientBytes...)
	serialized = append(serialized, borshU128(fee)...)
	serialized = append(serialized, borshU128(nativeFee)...)
	serialized = append(serialized, senderBytes...)
	serialized = append(serialized, borshString(msg)...)
	hash := sha256.Sum256(serialized)
	return hex.EncodeToString(hash[:]), nil
}

// verifyTransferAmount / getMinimumTransferableAmount (omni-bridge-sdk utils/decimals.js)
func normalizeAmount(amount *big.Int, fromDecimals, toDecimals int) *big.Int {
	if fromDecimals == toDecimals {
		return amount
	}
	if fromDecimals > toDecimals {
		return new(big.Int).Quo(amount, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(fromDecimals-toDecimals)), nil))
	}
	return new(big.Int).Mul(amount, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(toDecimals-fromDecimals)), nil))
}

func verifyTransferAmount(amount *big.Int, fee *big.Int, originDecimals, destinationDecimals int) bool {
	minDecimals := min(originDecimals, destinationDecimals)
	return normalizeAmount(new(big.Int).Sub(amount, fee), originDecimals, minDecimals).Sign() > 0
}

func getMinimumTransferableAmount(originDecimals, destinationDecimals int) *big.Int {
	minAmount := big.NewInt(1)
	if originDecimals > destinationDecimals {
		minAmount.Mul(minAmount, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(originDecimals-destinationDecimals)), nil))
	}
	return minAmount
}

type omniTokenDecimals struct {
	Decimals       int `json:"decimals"`
	OriginDecimals int `json:"origin_decimals"`
}

type omniFeeResponse struct {
	NativeTokenFee   json.Number  `json:"native_token_fee"`
	GasFee           *json.Number `json:"gas_fee"`
	ProtocolFee      *json.Number `json:"protocol_fee"`
	MinAmount        *string      `json:"min_amount"`
	InsufficientUtxo bool         `json:"insufficient_utxo"`
}

func safeBigInt(value json.Number) (*big.Int, error) {
	if parsed, ok := new(big.Int).SetString(value.String(), 10); ok {
		return parsed, nil
	}
	parsed, _, err := big.ParseFloat(value.String(), 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("Invalid BigInt value: %s", value)
	}
	result, _ := parsed.Int(nil)
	return result, nil
}

type omniBridge struct {
	storageDepositCache          *ttlCache[[2]*big.Int]
	destinationChainAddressCache *ttlCache[string]
	tokenDecimalsCache           *ttlCache[omniTokenDecimals]
}

func newOmniBridge() *omniBridge {
	return &omniBridge{
		storageDepositCache:          newTTLCache[[2]*big.Int](time.Hour),
		destinationChainAddressCache: newTTLCache[string](time.Hour),
		tokenDecimalsCache:           newTTLCache[omniTokenDecimals](time.Hour),
	}
}

func (b *omniBridge) route() string { return routeOmniBridge }

func (b *omniBridge) makeAssetInfo(assetID string) (parsedDefuseAssetID, string, bool, error) {
	parsed, err := parseDefuseAssetID(assetID)
	if err != nil {
		return parsedDefuseAssetID{}, "", false, err
	}
	if parsed.standard != "nep141" {
		return parsedDefuseAssetID{}, "", false, nil
	}
	kind, ok := parseOriginChain(parsed.contractID)
	if !ok {
		return parsedDefuseAssetID{}, "", false, nil
	}
	blockchain, ok := chainKindToCaip2(kind)
	if !ok {
		return parsedDefuseAssetID{}, "", false, nil
	}
	return parsed, blockchain, true, nil
}

func (b *omniBridge) supports(ctx context.Context, params withdrawalParams) (bool, error) {
	parsed, err := parseDefuseAssetID(params.AssetID)
	if err != nil {
		return false, err
	}
	if parsed.standard != "nep141" {
		return false, nil
	}
	if !validateOmniToken(parsed.contractID) {
		return false, nil
	}
	kind, ok := parseOriginChain(parsed.contractID)
	if !ok {
		return false, fmt.Errorf("Withdrawal of %s to its origin chain is not supported in Omni Bridge. Asset ID: %s", parsed.contractID, params.AssetID)
	}
	caip2Chain, ok := chainKindToCaip2(kind)
	if !ok {
		return false, fmt.Errorf("Withdrawal of %s to its origin chain is not supported in Omni Bridge. Asset ID: %s", parsed.contractID, params.AssetID)
	}
	destinationToken, err := b.getCachedDestinationTokenAddress(ctx, parsed.contractID, kind)
	if err != nil {
		return false, err
	}
	if destinationToken == "" {
		return false, fmt.Errorf("The token %s doesn't exist in destination chain %s", params.AssetID, caip2Chain)
	}
	return true, nil
}

type omniFtWithdrawPayload struct {
	Recipient      string `json:"recipient"`
	Fee            string `json:"fee"`
	NativeTokenFee string `json:"native_token_fee"`
	Msg            string `json:"msg,omitempty"`
}

type orderedTokenDiff struct {
	assetIn   string
	amountIn  string
	assetOut  string
	amountOut string
}

func (d orderedTokenDiff) MarshalJSON() ([]byte, error) {
	assetIn, _ := json.Marshal(d.assetIn)
	amountIn, _ := json.Marshal(d.amountIn)
	assetOut, _ := json.Marshal(d.assetOut)
	amountOut, _ := json.Marshal(d.amountOut)
	return []byte("{" + string(assetIn) + ":" + string(amountIn) + "," + string(assetOut) + ":" + string(amountOut) + "}"), nil
}

func (b *omniBridge) createWithdrawalIntents(params withdrawalParams, fee feeEstimation, referral string) ([]any, error) {
	parsed, blockchain, ok, err := b.makeAssetInfo(params.AssetID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("Asset %s is not supported by Omni Bridge", params.AssetID)
	}
	kind, ok := caip2ToChainKind(blockchain)
	if !ok {
		return nil, fmt.Errorf("Chain %s is not supported by Omni Bridge", blockchain)
	}

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

	relayerFee, err := getUnderlyingFee(fee, routeOmniBridge, "relayerFee")
	if err != nil {
		return nil, err
	}
	if relayerFee == nil || relayerFee.Sign() < 0 {
		return nil, fmt.Errorf("Invalid Omni bridge relayer fee: expected >= 0, got %s", relayerFee)
	}
	amount := new(big.Int).Set(params.Amount)
	var utxoMaxGasFee *big.Int
	if isUtxoChain(kind) {
		utxoMaxGasFee, err = getUnderlyingFee(fee, routeOmniBridge, "utxoMaxGasFee")
		if err != nil {
			return nil, err
		}
		utxoProtocolFee, err := getUnderlyingFee(fee, routeOmniBridge, "utxoProtocolFee")
		if err != nil {
			return nil, err
		}
		if utxoMaxGasFee == nil || utxoMaxGasFee.Sign() <= 0 {
			return nil, fmt.Errorf("Invalid Omni Bridge utxo max gas fee: expected > 0, got %s", utxoMaxGasFee)
		}
		if utxoProtocolFee == nil || utxoProtocolFee.Sign() <= 0 {
			return nil, fmt.Errorf("Invalid Omni Bridge utxo protocol fee: expected > 0, got %s", utxoProtocolFee)
		}
		amount.Add(amount, utxoMaxGasFee)
		amount.Add(amount, utxoProtocolFee)
	}
	storageDepositAmount, err := getUnderlyingFee(fee, routeOmniBridge, "storageDepositFee")
	if err != nil {
		return nil, err
	}

	// createWithdrawIntentsPrimitive (omni-bridge-utils.js)
	recipient := omniAddress(kind, params.DestinationAddress)
	msg := ""
	ftWithdrawPayload := omniFtWithdrawPayload{Recipient: recipient, Fee: "0", NativeTokenFee: relayerFee.String()}
	if isUtxoChain(kind) {
		if utxoMaxGasFee == nil || utxoMaxGasFee.Sign() <= 0 {
			return nil, fmt.Errorf("Invalid utxo max gas fee: expected > 0, got %s", utxoMaxGasFee)
		}
		msg, err = marshalJSONNoEscape(map[string]string{"MaxGasFee": utxoMaxGasFee.String()})
		if err != nil {
			return nil, err
		}
		ftWithdrawPayload.Msg = msg
	}
	if relayerFee.Sign() > 0 {
		depositForAccountID, err := calculateStorageAccountID(
			"near:"+parsed.contractID,
			amount,
			recipient,
			big.NewInt(0),
			relayerFee,
			"near:"+constants.NearIntentsContractID,
			msg,
		)
		if err != nil {
			return nil, err
		}
		intents = append(intents, struct {
			DepositForAccountID string `json:"deposit_for_account_id"`
			Amount              string `json:"amount"`
			ContractID          string `json:"contract_id"`
			Intent              string `json:"intent"`
		}{
			DepositForAccountID: depositForAccountID,
			Amount:              relayerFee.String(),
			ContractID:          omniBridgeContract,
			Intent:              "storage_deposit",
		})
	}
	ftWithdrawMsg, err := marshalJSONNoEscape(ftWithdrawPayload)
	if err != nil {
		return nil, err
	}
	storageDeposit := ""
	if storageDepositAmount != nil && storageDepositAmount.Sign() > 0 {
		storageDeposit = storageDepositAmount.String()
	}
	intents = append(intents, struct {
		Intent         string `json:"intent"`
		Token          string `json:"token"`
		ReceiverID     string `json:"receiver_id"`
		Amount         string `json:"amount"`
		StorageDeposit string `json:"storage_deposit,omitempty"`
		Msg            string `json:"msg"`
		MinGas         string `json:"min_gas"`
	}{
		Intent:         "ft_withdraw",
		Token:          parsed.contractID,
		ReceiverID:     omniBridgeContract,
		Amount:         amount.String(),
		StorageDeposit: storageDeposit,
		Msg:            ftWithdrawMsg,
		MinGas:         omniMinGasAmount,
	})
	return intents, nil
}

func (b *omniBridge) getFee(ctx context.Context, recipient, tokenAddress string, amount *big.Int) (omniFeeResponse, error) {
	query := url.Values{}
	query.Set("sender", omniAddress(chainKindNear, constants.NearIntentsContractID))
	query.Set("recipient", recipient)
	query.Set("token", tokenAddress)
	query.Set("amount", amount.String())
	feeCtx, cancel := context.WithTimeout(ctx, omniWithdrawalAPIFeeRequestTimeout)
	defer cancel()
	response, err := requestOnce(feeCtx, http.MethodGet, omniBridgeAPIBaseURL+"/api/v3/transfer-fee?"+query.Encode(), nil, omniWithdrawalAPIFeeRequestTimeout)
	if err != nil {
		if errors.Is(feeCtx.Err(), context.DeadlineExceeded) {
			return omniFeeResponse{}, errors.New("Omni bridge withdrawal fee request timed out.")
		}
		return omniFeeResponse{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(response))
	decoder.UseNumber()
	var fee omniFeeResponse
	if err := decoder.Decode(&fee); err != nil {
		return omniFeeResponse{}, err
	}
	return fee, nil
}

func (b *omniBridge) validateWithdrawal(ctx context.Context, assetID string, amount *big.Int, destinationAddress string, fee feeEstimation) error {
	if fee.amount.Sign() <= 0 {
		return fmt.Errorf("Invalid Omni Bridge fee: expected > 0, got %s", fee.amount)
	}
	parsed, blockchain, ok, err := b.makeAssetInfo(assetID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Asset %s is not supported by Omni Bridge", assetID)
	}
	if !validateAddress(destinationAddress, blockchain) {
		return fmt.Errorf("Invalid destination address. Details: Destination address %s is not valid for withdrawal to %s.", destinationAddress, blockchain)
	}
	kind, ok := caip2ToChainKind(blockchain)
	if !ok {
		return fmt.Errorf("Chain %s is not supported by Omni Bridge", blockchain)
	}
	destTokenAddress, err := b.getCachedDestinationTokenAddress(ctx, parsed.contractID, kind)
	if err != nil {
		return err
	}
	if destTokenAddress == "" {
		return fmt.Errorf("The token %s doesn't exist in destination chain %s", assetID, blockchain)
	}
	decimals, found, err := b.getCachedTokenDecimals(ctx, destTokenAddress)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Failed to retrieve token decimals for address %s via OmniBridge contract. \n  Ensure the token is supported and the address is correct.", destTokenAddress)
	}
	if !verifyTransferAmount(amount, big.NewInt(0), decimals.OriginDecimals, decimals.Decimals) {
		return fmt.Errorf("Withdrawal amount is below minimum required by the bridge. Asset ID: %s Minimum amount: %s Requested amount: %s", assetID, getMinimumTransferableAmount(decimals.OriginDecimals, decimals.Decimals), amount)
	}

	storageBalance, err := callFunction[*struct {
		Total     string `json:"total"`
		Available string `json:"available"`
	}](ctx, omniBridgeContract, "storage_balance_of", map[string]string{"account_id": constants.NearIntentsContractID})
	if err != nil {
		return err
	}
	intentsNearStorageBalance := big.NewInt(0)
	if storageBalance != nil && *storageBalance != nil {
		intentsNearStorageBalance = mustBigInt((*storageBalance).Available)
	}
	if intentsNearStorageBalance.Cmp(mustBigInt(minAllowedStorageBalanceForIntentsNear)) <= 0 {
		return fmt.Errorf("Omni storage balance of intents contract is too low to complete this transaction safely. Current balance: %s", intentsNearStorageBalance)
	}

	if !isUtxoChain(kind) {
		relayerFee, err := getUnderlyingFee(fee, routeOmniBridge, "relayerFee")
		if err != nil {
			return err
		}
		if relayerFee == nil || relayerFee.Sign() <= 0 {
			return fmt.Errorf("Invalid Omni Bridge relayer fee for non UTXO chain withdrawal: expected > 0, got %s", relayerFee)
		}
		return nil
	}

	apiFee, err := b.getFee(ctx, omniAddress(kind, destinationAddress), omniAddress(chainKindNear, parsed.contractID), amount)
	if err != nil {
		return err
	}
	if apiFee.InsufficientUtxo {
		return fmt.Errorf("Insufficient utxo for withdrawal to a utxo chain. Chain: %s", blockchain)
	}
	if apiFee.MinAmount == nil || mustBigInt(*apiFee.MinAmount).Sign() <= 0 {
		return fmt.Errorf("Invalid min amount value for a UTXO chain withdrawal: expected > 0, got %v", apiFee.MinAmount)
	}
	minAmount := mustBigInt(*apiFee.MinAmount)
	utxoMaxGasFee, err := getUnderlyingFee(fee, routeOmniBridge, "utxoMaxGasFee")
	if err != nil {
		return err
	}
	utxoProtocolFee, err := getUnderlyingFee(fee, routeOmniBridge, "utxoProtocolFee")
	if err != nil {
		return err
	}
	if utxoMaxGasFee == nil || utxoMaxGasFee.Sign() <= 0 {
		return fmt.Errorf("Invalid Omni Bridge utxo max gas fee: expected > 0, got %s", utxoMaxGasFee)
	}
	if utxoProtocolFee == nil || utxoProtocolFee.Sign() <= 0 {
		return fmt.Errorf("Invalid Omni Bridge utxo protocol fee: expected > 0, got %s", utxoProtocolFee)
	}
	actualAmountWithFee := new(big.Int).Add(amount, utxoMaxGasFee)
	actualAmountWithFee.Add(actualAmountWithFee, utxoProtocolFee)
	if actualAmountWithFee.Cmp(minAmount) < 0 {
		return fmt.Errorf("Withdrawal amount is below minimum required by the bridge. Asset ID: %s Minimum amount: %s Requested amount: %s", assetID, minAmount, actualAmountWithFee)
	}
	return nil
}

func (b *omniBridge) estimateWithdrawalFee(ctx context.Context, params withdrawalParams) (feeEstimation, error) {
	parsed, blockchain, ok, err := b.makeAssetInfo(params.AssetID)
	if err != nil {
		return feeEstimation{}, err
	}
	if !ok {
		return feeEstimation{}, fmt.Errorf("Asset %s is not supported by Omni Bridge", params.AssetID)
	}
	kind, ok := caip2ToChainKind(blockchain)
	if !ok {
		return feeEstimation{}, fmt.Errorf("Chain %s is not supported by Omni Bridge", blockchain)
	}
	apiFee, err := b.getFee(ctx, omniAddress(kind, params.DestinationAddress), omniAddress(chainKindNear, parsed.contractID), params.Amount)
	if err != nil {
		return feeEstimation{}, err
	}
	nativeTokenFee, err := safeBigInt(apiFee.NativeTokenFee)
	if err != nil || nativeTokenFee.Sign() < 0 {
		return feeEstimation{}, fmt.Errorf("Invalid fee value. Asset ID: %s Fee: %s", params.AssetID, apiFee.NativeTokenFee)
	}
	underlyingFees := map[string]*big.Int{"relayerFee": nativeTokenFee, "storageDepositFee": big.NewInt(0)}
	totalAmountToQuote := new(big.Int).Set(nativeTokenFee)

	storageBalances, err := b.getCachedStorageDepositValue(ctx, parsed.contractID)
	if err != nil {
		return feeEstimation{}, err
	}
	storageDepositFee := new(big.Int).Sub(storageBalances[0], storageBalances[1])
	if storageDepositFee.Sign() > 0 {
		totalAmountToQuote.Add(totalAmountToQuote, storageDepositFee)
		underlyingFees["storageDepositFee"] = storageDepositFee
	}
	if params.AssetID == nearNativeAssetID {
		return feeEstimation{amount: totalAmountToQuote, underlyingFees: map[string]map[string]*big.Int{routeOmniBridge: underlyingFees}}, nil
	}

	amount := big.NewInt(0)
	var quote *relayQuote
	if totalAmountToQuote.Sign() > 0 {
		feeQuote, err := getFeeQuote(ctx, totalAmountToQuote, nearNativeAssetID, params.AssetID)
		if err != nil {
			return feeEstimation{}, err
		}
		quote = &feeQuote
		amount.Add(amount, mustBigInt(feeQuote.AmountIn))
	}
	if isUtxoChain(kind) {
		if apiFee.GasFee == nil || apiFee.ProtocolFee == nil {
			return feeEstimation{}, errors.New("Invalid Omni Bridge utxo gas fee: expected > 0, got null")
		}
		gasFee, err := safeBigInt(*apiFee.GasFee)
		if err != nil || gasFee.Sign() <= 0 {
			return feeEstimation{}, fmt.Errorf("Invalid Omni Bridge utxo gas fee: expected > 0, got %s", *apiFee.GasFee)
		}
		protocolFee, err := safeBigInt(*apiFee.ProtocolFee)
		if err != nil || protocolFee.Sign() <= 0 {
			return feeEstimation{}, fmt.Errorf("Invalid Omni Bridge utxo protocol fee: expected > 0, got %s", *apiFee.ProtocolFee)
		}
		amount.Add(amount, gasFee)
		amount.Add(amount, protocolFee)
		underlyingFees["utxoMaxGasFee"] = gasFee
		underlyingFees["utxoProtocolFee"] = protocolFee
	}
	return feeEstimation{amount: amount, quote: quote, underlyingFees: map[string]map[string]*big.Int{routeOmniBridge: underlyingFees}}, nil
}

func (b *omniBridge) createWithdrawalIdentifier(params withdrawalParams, index int, txHash string) (withdrawalIdentifier, error) {
	_, blockchain, ok, err := b.makeAssetInfo(params.AssetID)
	if err != nil {
		return withdrawalIdentifier{}, err
	}
	if !ok {
		return withdrawalIdentifier{}, fmt.Errorf("Asset %s is not supported by Omni Bridge", params.AssetID)
	}
	return withdrawalIdentifier{landingChain: blockchain, index: index, withdrawalParams: params, txHash: txHash}, nil
}

func (b *omniBridge) describeWithdrawal(ctx context.Context, wid withdrawalIdentifier) (withdrawalStatus, error) {
	response, err := request(ctx, http.MethodGet, omniBridgeAPIBaseURL+"/api/v3/transfers/transfer?transaction_hash="+url.QueryEscape(wid.txHash), nil, defaultRequestTimeout, retryOptions{maxAttempts: 1})
	if err != nil {
		return withdrawalStatus{}, err
	}
	var transfers []struct {
		Finalised *struct {
			EVMLog *struct {
				TransactionHash string `json:"transaction_hash"`
			} `json:"EVMLog"`
			Solana *struct {
				Signature string `json:"signature"`
			} `json:"Solana"`
			UtxoLog *struct {
				TransactionHash string `json:"transaction_hash"`
			} `json:"UtxoLog"`
		} `json:"finalised"`
		TransferMessage *struct {
			Recipient string `json:"recipient"`
		} `json:"transfer_message"`
	}
	if err := json.Unmarshal(response, &transfers); err != nil {
		return withdrawalStatus{}, err
	}
	if wid.index >= len(transfers) || transfers[wid.index].TransferMessage == nil {
		return withdrawalStatus{status: "pending"}, nil
	}
	transfer := transfers[wid.index]
	// getChain returns undefined for an unknown prefix, which falls through to "completed"
	destinationChain, known := getOmniChain(transfer.TransferMessage.Recipient)
	txHash := ""
	switch {
	case !known:
		return withdrawalStatus{status: "completed"}, nil
	case destinationChain == chainKindEth || destinationChain == chainKindBase || destinationChain == chainKindArb || destinationChain == chainKindBnb:
		if transfer.Finalised != nil && transfer.Finalised.EVMLog != nil {
			txHash = transfer.Finalised.EVMLog.TransactionHash
		}
	case destinationChain == chainKindSol:
		if transfer.Finalised != nil && transfer.Finalised.Solana != nil {
			txHash = transfer.Finalised.Solana.Signature
		}
	case destinationChain == chainKindBtc:
		if transfer.Finalised != nil && transfer.Finalised.UtxoLog != nil {
			txHash = transfer.Finalised.UtxoLog.TransactionHash
		}
	default:
		return withdrawalStatus{status: "completed"}, nil
	}
	if txHash == "" {
		return withdrawalStatus{status: "pending"}, nil
	}
	return withdrawalStatus{status: "completed"}, nil
}

func (b *omniBridge) getCachedStorageDepositValue(ctx context.Context, contractID string) ([2]*big.Int, error) {
	if cached, ok := b.storageDepositCache.get(contractID); ok {
		return cached, nil
	}
	result, err := getNearNep141StorageBalances(ctx, contractID, omniBridgeContract)
	if err != nil {
		return [2]*big.Int{}, err
	}
	if result[1].Cmp(result[0]) >= 0 {
		b.storageDepositCache.set(contractID, result)
	}
	return result, nil
}

func (b *omniBridge) getCachedDestinationTokenAddress(ctx context.Context, contractID string, kind omniChainKind) (string, error) {
	key := fmt.Sprintf("%d:%s", kind, contractID)
	if cached, ok := b.destinationChainAddressCache.get(key); ok {
		return cached, nil
	}
	token, err := callFunction[*string](ctx, omniBridgeContract, "get_bridged_token", struct {
		Chain   string `json:"chain"`
		Address string `json:"address"`
	}{Chain: chainKindNames[kind], Address: omniAddress(chainKindNear, contractID)})
	if err != nil {
		return "", err
	}
	if token == nil || *token == nil {
		return "", nil
	}
	b.destinationChainAddressCache.set(key, **token)
	return **token, nil
}

func (b *omniBridge) getCachedTokenDecimals(ctx context.Context, address string) (omniTokenDecimals, bool, error) {
	if cached, ok := b.tokenDecimalsCache.get(address); ok {
		return cached, true, nil
	}
	if kind, _ := getOmniChain(address); kind == chainKindNear {
		return omniTokenDecimals{}, false, errors.New("Token decimals cannot be queried using NEAR addresses. Use the token's foreign chain representation (e.g., eth:0x...) to query decimals.")
	}
	decimals, err := callFunction[*omniTokenDecimals](ctx, omniBridgeContract, "get_token_decimals", map[string]string{"address": address})
	if err != nil {
		return omniTokenDecimals{}, false, err
	}
	if decimals == nil || *decimals == nil {
		return omniTokenDecimals{}, false, nil
	}
	b.tokenDecimalsCache.set(address, **decimals)
	return **decimals, true, nil
}

// ---------------------------------------------------------------------------------------------------
// intents-sdk: IntentsSDK (sdk.js) and core/withdrawal-watcher.js
// ---------------------------------------------------------------------------------------------------

type intentsSDK struct {
	referral    string
	bridges     []withdrawalBridge
	saltManager *saltManager
}

func newIntentsSDK(referral string) *intentsSDK {
	return &intentsSDK{
		referral:    referral,
		bridges:     []withdrawalBridge{newPoaBridge(), newHotBridge(), newOmniBridge(), newDirectBridge()}, // sdk.js order minus IntentsBridge/AuroraEngine (routeConfig only)
		saltManager: &saltManager{},
	}
}

func (s *intentsSDK) findBridge(ctx context.Context, params withdrawalParams) (withdrawalBridge, error) {
	for _, bridge := range s.bridges {
		supported, err := bridge.supports(ctx, params)
		if err != nil {
			return nil, err
		}
		if supported {
			return bridge, nil
		}
	}
	return nil, fmt.Errorf("Cannot determine bridge for withdrawal = {\"assetId\":%q,\"amount\":\"%s\",\"destinationAddress\":%q,\"feeInclusive\":%t}", params.AssetID, params.Amount, params.DestinationAddress, params.FeeInclusive)
}

func (s *intentsSDK) estimateWithdrawalFee(ctx context.Context, params withdrawalParams) (feeEstimation, error) {
	bridge, err := s.findBridge(ctx, params)
	if err != nil {
		return feeEstimation{}, err
	}
	fee, err := bridge.estimateWithdrawalFee(ctx, params)
	if err != nil {
		return feeEstimation{}, err
	}
	if params.FeeInclusive && params.Amount.Cmp(fee.amount) <= 0 {
		return feeEstimation{}, fmt.Errorf("Amount too small to pay fee. Required fee: %s Withdrawal amount: %s", fee.amount, params.Amount)
	}
	return fee, nil
}

func (s *intentsSDK) createWithdrawalIntents(ctx context.Context, params withdrawalParams, fee feeEstimation) ([]any, error) {
	bridge, err := s.findBridge(ctx, params)
	if err != nil {
		return nil, err
	}
	actualAmount := params.Amount
	if params.FeeInclusive {
		actualAmount = new(big.Int).Sub(params.Amount, fee.amount)
	}
	if err := bridge.validateWithdrawal(ctx, params.AssetID, actualAmount, params.DestinationAddress, fee); err != nil {
		return nil, err
	}
	actualParams := params
	actualParams.Amount = actualAmount
	return bridge.createWithdrawalIntents(actualParams, fee, s.referral)
}

func (s *intentsSDK) signAndSendIntent(ctx context.Context, signer *intentSignerNEP413, intents []any, quoteHashes []string) (string, error) {
	send := func(salt []byte) (string, error) {
		payload, err := defaultIntentPayloadFactory(salt, intents, constants.NearIntentsContractID)
		if err != nil {
			return "", err
		}
		signed, err := signer.signIntent(payload)
		if err != nil {
			return "", err
		}
		tickets, err := publishIntents(ctx, []multiPayload{signed}, quoteHashes)
		if err != nil {
			return "", err
		}
		if len(tickets) == 0 {
			return "", errors.New("Failed to publish intent.")
		}
		return tickets[0], nil
	}

	// withSaltRetry
	salt, err := s.saltManager.getCachedSalt(ctx)
	if err != nil {
		return "", err
	}
	ticket, err := send(salt)
	var publishErr relayPublishError
	if err == nil || !errors.As(err, &publishErr) || publishErr.code != "INVALID_SALT" {
		return ticket, err
	}
	salt, err = s.saltManager.refresh(ctx)
	if err != nil {
		return "", err
	}
	return send(salt)
}

func (s *intentsSDK) signAndSendWithdrawalIntent(ctx context.Context, signer *intentSignerNEP413, params withdrawalParams, fee feeEstimation) (string, error) {
	intents, err := s.createWithdrawalIntents(ctx, params, fee)
	if err != nil {
		return "", err
	}
	quoteHashes := []string{}
	if fee.quote != nil {
		quoteHashes = append(quoteHashes, fee.quote.QuoteHash)
	}
	return s.signAndSendIntent(ctx, signer, intents, quoteHashes)
}

func (s *intentsSDK) waitForWithdrawalCompletion(ctx context.Context, params withdrawalParams, intentTxHash string) error {
	bridge, err := s.findBridge(ctx, params)
	if err != nil {
		return errors.New("Bridge adapter not found")
	}
	wid, err := bridge.createWithdrawalIdentifier(params, 0, intentTxHash)
	if err != nil {
		return err
	}

	// watchWithdrawal
	const maxConsecutiveErrors = 3
	consecutiveErrors := 0
	_, err = poll(ctx, getWithdrawalStatsForChain(wid.landingChain, bridge.route()), func() (struct{}, bool, error) {
		status, err := bridge.describeWithdrawal(ctx, wid)
		if err != nil {
			if ctx.Err() != nil {
				return struct{}{}, false, ctx.Err()
			}
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				return struct{}{}, false, fmt.Errorf("Withdrawal watch failed: %w", err)
			}
			return struct{}{}, false, nil
		}
		consecutiveErrors = 0
		switch status.status {
		case "completed":
			return struct{}{}, true, nil
		case "failed":
			return struct{}{}, false, fmt.Errorf("Withdrawal failed: %s", status.reason)
		default:
			return struct{}{}, false, nil
		}
	})
	var timeoutErr pollTimeoutError
	if errors.As(err, &timeoutErr) {
		return fmt.Errorf("Withdrawal watch failed: %w", err)
	}
	return err
}

// processWithdrawal (sdk.js) is split in two so a caller can release its lock before the long wait.
// submitWithdrawal estimates the fee, then signs and publishes the intent. It takes the signer per call instead of
// setIntentSigner on the shared SDK, so concurrent claims for different accounts cannot sign with each other's key.
func (s *intentsSDK) submitWithdrawal(ctx context.Context, signer *intentSignerNEP413, params withdrawalParams) (string, error) {
	if signer == nil {
		return "", errors.New("intent signer is not provided")
	}

	fee, err := s.estimateWithdrawalFee(ctx, params)
	if err != nil {
		return "", err
	}
	return s.signAndSendWithdrawalIntent(ctx, signer, params, fee)
}

// awaitWithdrawal is the slow half of processWithdrawal: it can poll for hours and needs no account state.
func (s *intentsSDK) awaitWithdrawal(ctx context.Context, params withdrawalParams, intentHash string) error {
	intentTxHash, err := waitForIntentSettlement(ctx, intentHash)
	if err != nil {
		return err
	}
	return s.waitForWithdrawalCompletion(ctx, params, intentTxHash)
}
