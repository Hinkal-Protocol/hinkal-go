package transactions

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

// ---------------------------------------------------------------------------------------------------
// intents-sdk: HotBridge (bridges/hot-bridge), with the parts of @hot-labs/omni-sdk 2.24.7 it runs:
// toOmni/fromOmni, encodeReceiver, getGaslessWithdrawFee, buildGaslessWithdrawIntent,
// getGaslessWithdrawStatus, NearBridge.parseWithdrawalNonces and the HOT API withdrawal-hash fallback.
// Only the EVM HOT chains are ported; TON and Stellar need address codecs Hinkal never uses.
// ---------------------------------------------------------------------------------------------------

const (
	routeHotBridge        = "hot_bridge"
	caip2TON              = "tvm:-239"
	caip2Stellar          = "stellar:pubnet"
	omniHotV2             = "v2_1.omni.hot.tg"
	hotRefuelContract     = "bridge-refuel.hot.tg"
	hotMinGasAmount       = "91300000000000"
	hotStatusCanceled     = "CANCELED"
	hotEvmNativeAddress   = "11111111111111111111"
	hotFeeRequestTimeout  = 3 * time.Second  // withTimeout outside the browser
	hotAPIFallbackTimeout = 5 * time.Second  // HotBridge.API_FALLBACK_TIMEOUT_MS
	hotNearRPCTimeout     = 10 * time.Second // HotBridge.NEAR_RPC_TIMEOUT_MS
	hotDefaultEvmGas      = 1_000_000        // defaultEvmWithdrawFee
)

// HotBridgeChains (hot-bridge-chains.js)
var hotBridgeChains = []string{
	caip2Polygon, caip2BNB, caip2TON, caip2Optimism, caip2Avalanche, caip2Monad, caip2Stellar,
	caip2LayerX, caip2Adi, caip2Scroll, caip2Plasma,
}

// nativeTokenMapping (hot-bridge-utils.js)
var hotNativeTokenMapping = map[string]string{
	caip2BNB:       "nep245:v2_1.omni.hot.tg:56_11111111111111111111",
	caip2Polygon:   "nep245:v2_1.omni.hot.tg:137_11111111111111111111",
	caip2Monad:     "nep245:v2_1.omni.hot.tg:143_11111111111111111111",
	caip2TON:       "nep245:v2_1.omni.hot.tg:1117_",
	caip2Optimism:  "nep245:v2_1.omni.hot.tg:10_11111111111111111111",
	caip2Avalanche: "nep245:v2_1.omni.hot.tg:43114_11111111111111111111",
	caip2Stellar:   "nep245:v2_1.omni.hot.tg:1100_111bzQBB5v7AhLyPMDwS8uJgQV24KaAPXtwyVWu2KXbbfQU6NXRCz",
	caip2LayerX:    "nep245:v2_1.omni.hot.tg:196_11111111111111111111",
	caip2Adi:       "nep245:v2_1.omni.hot.tg:36900_11111111111111111111",
	caip2Scroll:    "nep245:v2_1.omni.hot.tg:534352_11111111111111111111",
	caip2Plasma:    "nep245:v2_1.omni.hot.tg:9745_11111111111111111111",
}

// caip2NetworkIdMapping (hot-bridge-utils.js); omni-sdk Network ids
var hotCaip2NetworkID = map[string]int{
	caip2BNB: 56, caip2Polygon: 137, caip2Monad: 143, caip2TON: 1111, caip2Optimism: 10, caip2Avalanche: 43114,
	caip2Stellar: 1100, caip2LayerX: 196, caip2Adi: 36900, caip2Scroll: 534352, caip2Plasma: 9745,
}

// Bridge.withdrawFees (omni-sdk bridge.js), gas limit per network
var hotWithdrawGasLimits = map[int]int64{
	56: 200_000, 1: 200_000, 137: 200_000, 43114: 200_000, 8453: 200_000, 10: 200_000, 196: 200_000, 143: 200_000,
}

// PUBLIC_EVM_RPC_URLS (constants/public-rpc-urls.js); the TS IntentsSDK is built without rpc overrides
var hotPublicEvmRPCURLs = map[string]string{
	caip2BNB:       "https://bsc-rpc.publicnode.com",
	caip2Polygon:   "https://polygon-bor-rpc.publicnode.com",
	caip2Monad:     "https://rpc.monad.xyz",
	caip2Optimism:  "https://optimism-rpc.publicnode.com",
	caip2Avalanche: "https://avalanche-c-chain-rpc.publicnode.com",
	caip2LayerX:    "https://rpc.xlayer.tech",
	caip2Adi:       "https://rpc.adifoundation.ai",
	caip2Plasma:    "https://rpc.plasma.to",
	caip2Scroll:    "https://rpc.scroll.io",
}

// OmniApi default endpoints (omni-sdk api.js)
var hotAPIEndpoints = []string{"https://api0.herewallet.app", "https://api2.herewallet.app"}

var hotMemoNonceRegex = regexp.MustCompile(`"memo":"(\d+)"`)

func hotNetworkIDToCAIP2(network int) (string, error) {
	for caip2, id := range hotCaip2NetworkID {
		if id == network {
			return caip2, nil
		}
	}
	return "", fmt.Errorf("Unsupported HOT Bridge chain = %d", network)
}

func hotBlockchainInvariant(blockchain string) error {
	for _, chain := range hotBridgeChains {
		if chain == blockchain {
			return nil
		}
	}
	return fmt.Errorf("%s is not a valid HOT Bridge blockchain. Supported values: %s", blockchain, strings.Join(hotBridgeChains, ","))
}

// isHex (lib/hex.js): non 0x-prefixed, even-length hex
func isHex(value string) bool {
	return len(value)%2 == 0 && hexPattern.MatchString(value)
}

// decodeTokenAddress (omni-sdk utils.js), EVM branch
func hotDecodeEvmTokenAddress(addr string) (string, error) {
	switch addr {
	case "", "1", hotEvmNativeAddress, "11111111111111111111111111111111", "111bzQBB5v7AhLyPMDwS8uJgQV24KaAPXtwyVWu2KXbbfQU6NXRCz":
		return "native", nil
	}
	decoded, err := base58.Decode(addr)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(decoded), nil
}

// toOmni(chain, address) (omni-sdk utils.js), EVM branch
func hotToOmniEvm(network int, address string) (string, error) {
	if address == "native" {
		return fmt.Sprintf("%d_%s", network, hotEvmNativeAddress), nil
	}
	bytes, err := hex.DecodeString(strings.TrimPrefix(address, "0x"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d_%s", network, base58.Encode(bytes)), nil
}

// encodeReceiver (omni-sdk utils.js), EVM branch
func hotEncodeEvmReceiver(address string) (string, error) {
	bytes, err := hex.DecodeString(strings.TrimPrefix(address, "0x"))
	if err != nil {
		return "", err
	}
	return base58.Encode(bytes), nil
}

type hotAssetInfo struct {
	blockchain string
	native     bool
	address    string
}

type hotBridge struct {
	noncesCache *ttlCache[[]*big.Int]
}

func newHotBridge() *hotBridge {
	return &hotBridge{noncesCache: newTTLCache[[]*big.Int](time.Hour)}
}

func (b *hotBridge) route() string { return routeHotBridge }

// parseAssetID returns ok=false for assets that are not HOT (contractId != v2_1.omni.hot.tg).
func (b *hotBridge) parseAssetID(assetID string) (hotAssetInfo, bool, error) {
	parsed, err := parseDefuseAssetID(assetID)
	if err != nil {
		return hotAssetInfo{}, false, err
	}
	if parsed.contractID != omniHotV2 {
		return hotAssetInfo{}, false, nil
	}
	if parsed.standard != "nep245" {
		return hotAssetInfo{}, false, fmt.Errorf("Should start with \"nep245:\". Asset ID: %s", assetID)
	}

	// fromOmni(tokenId).split(":")
	tokenID := parsed.tokenID
	parts := strings.Split(tokenID, "_")
	if len(parts) < 2 { // fromOmni maps an id without "_" to NEAR (1010), which is not a HOT chain
		return hotAssetInfo{}, false, fmt.Errorf("Asset belongs to unknown blockchain. Asset ID: %s", assetID)
	}
	network, err := strconv.Atoi(parts[0])
	if err != nil { // +"abc" is NaN, which hotNetworkIdToCAIP2 rejects
		return hotAssetInfo{}, false, fmt.Errorf("Asset belongs to unknown blockchain. Asset ID: %s", assetID)
	}
	if network == 1117 { // OMNI_TON -> TON
		network = 1111
	}
	blockchain, err := hotNetworkIDToCAIP2(network)
	if err != nil {
		return hotAssetInfo{}, false, fmt.Errorf("Asset belongs to unknown blockchain. Asset ID: %s", assetID)
	}

	if !strings.HasPrefix(blockchain, "eip155:") { // TON/Stellar address codecs are not ported
		return hotAssetInfo{}, false, fmt.Errorf("HOT Bridge withdrawal to %s is not supported in the Go SDK", blockchain)
	}
	address, err := hotDecodeEvmTokenAddress(parts[1])
	if err != nil {
		return hotAssetInfo{}, false, fmt.Errorf("Asset has invalid token id format. Asset ID: %s", assetID)
	}
	if address == "native" {
		return hotAssetInfo{blockchain: blockchain, native: true}, true, nil
	}
	return hotAssetInfo{blockchain: blockchain, address: address}, true, nil
}

func (b *hotBridge) supports(_ context.Context, params withdrawalParams) (bool, error) {
	_, ok, err := b.parseAssetID(params.AssetID)
	return ok, err
}

func (b *hotBridge) mustAssetInfo(assetID string) (hotAssetInfo, error) {
	info, ok, err := b.parseAssetID(assetID)
	if err != nil {
		return hotAssetInfo{}, err
	}
	if !ok {
		return hotAssetInfo{}, errors.New("Asset is not supported")
	}
	return info, nil
}

func (b *hotBridge) createWithdrawalIntents(params withdrawalParams, fee feeEstimation, referral string) ([]any, error) {
	info, err := b.mustAssetInfo(params.AssetID)
	if err != nil {
		return nil, err
	}

	intents := []any{}
	feeAmount, err := getUnderlyingFee(fee, routeHotBridge, "relayerFee")
	if err != nil {
		return nil, err
	}
	blockNumber, err := getUnderlyingFee(fee, routeHotBridge, "blockNumber")
	if err != nil {
		return nil, err
	}
	if feeAmount == nil || blockNumber == nil {
		return nil, errors.New("Missing HOT bridge relayerFee or blockNumber in fee estimation")
	}
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

	amount := new(big.Int).Set(params.Amount)
	if info.native {
		amount.Add(amount, feeAmount)
	}

	// buildGaslessWithdrawIntent (omni-sdk bridge.js)
	network := hotCaip2NetworkID[info.blockchain]
	token := "native"
	if !info.native {
		token = info.address
	}
	tokenAddress, err := hotToOmniEvm(network, token)
	if err != nil {
		return nil, err
	}
	feeTokenAddress, err := hotToOmniEvm(network, "native")
	if err != nil {
		return nil, err
	}
	receiver, err := hotEncodeEvmReceiver(params.DestinationAddress)
	if err != nil {
		return nil, err
	}
	withFee := tokenAddress != feeTokenAddress && feeAmount.Sign() > 0
	amounts := []string{amount.String()}
	tokenIDs := []string{tokenAddress}
	if withFee {
		amounts = append(amounts, feeAmount.String())
		tokenIDs = append(tokenIDs, feeTokenAddress)
	}
	msg, err := marshalJSONNoEscape(struct {
		ReceiverID   string `json:"receiver_id"`
		AmountNative string `json:"amount_native"`
		BlockNumber  int64  `json:"block_number"`
	}{ReceiverID: receiver, AmountNative: feeAmount.String(), BlockNumber: blockNumber.Int64()})
	if err != nil {
		return nil, err
	}

	return append(intents, struct {
		Intent     string   `json:"intent"`
		ReceiverID string   `json:"receiver_id"`
		Amounts    []string `json:"amounts"`
		TokenIDs   []string `json:"token_ids"`
		Token      string   `json:"token"`
		Msg        string   `json:"msg"`
		MinGas     string   `json:"min_gas"`
	}{
		Intent:     "mt_withdraw",
		ReceiverID: hotRefuelContract,
		Amounts:    amounts,
		TokenIDs:   tokenIDs,
		Token:      omniHotV2,
		Msg:        msg,
		MinGas:     hotMinGasAmount,
	}), nil
}

func (b *hotBridge) validateWithdrawal(_ context.Context, assetID string, _ *big.Int, destinationAddress string, _ feeEstimation) error {
	info, err := b.mustAssetInfo(assetID)
	if err != nil {
		return err
	}
	if err := hotBlockchainInvariant(info.blockchain); err != nil {
		return err
	}
	if !validateAddress(destinationAddress, info.blockchain) {
		return fmt.Errorf("Invalid destination address. Details: Destination address %s is not valid for withdrawal to %s.", destinationAddress, info.blockchain)
	}
	return nil
}

// getGaslessWithdrawFee (omni-sdk bridge.js), EVM branch: eth_gasPrice * 1.3 * gasLimit and the latest block.
func hotGetGaslessWithdrawFee(ctx context.Context, blockchain string, network int) (*big.Int, *big.Int, error) {
	rpcURL, ok := hotPublicEvmRPCURLs[blockchain]
	if !ok {
		return nil, nil, fmt.Errorf("No rpc for chain %d", network)
	}
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, nil, err
	}
	blockNumber, err := client.BlockNumber(ctx)
	if err != nil {
		return nil, nil, err
	}
	gasLimit, ok := hotWithdrawGasLimits[network]
	if !ok {
		gasLimit = hotDefaultEvmGas
	}
	fee := new(big.Int).Div(new(big.Int).Mul(gasPrice, big.NewInt(130)), big.NewInt(100))
	return fee.Mul(fee, big.NewInt(gasLimit)), new(big.Int).SetUint64(blockNumber), nil
}

func (b *hotBridge) estimateWithdrawalFee(ctx context.Context, params withdrawalParams) (feeEstimation, error) {
	info, err := b.mustAssetInfo(params.AssetID)
	if err != nil {
		return feeEstimation{}, err
	}
	if err := hotBlockchainInvariant(info.blockchain); err != nil {
		return feeEstimation{}, err
	}
	feeAssetID := hotNativeTokenMapping[info.blockchain]

	feeCtx, cancel := context.WithTimeout(ctx, hotFeeRequestTimeout)
	feeAmount, blockNumber, err := hotGetGaslessWithdrawFee(feeCtx, info.blockchain, hotCaip2NetworkID[info.blockchain])
	cancel()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return feeEstimation{}, errors.New("Hot bridge withdrawal fee request timed out.")
		}
		return feeEstimation{}, err
	}
	if info.blockchain == caip2Plasma && params.AssetID != feeAssetID {
		feeAmount.Mul(feeAmount, big.NewInt(20))
	}

	underlyingFees := map[string]map[string]*big.Int{routeHotBridge: {"relayerFee": feeAmount, "blockNumber": blockNumber}}
	if params.AssetID == feeAssetID || feeAmount.Sign() == 0 {
		return feeEstimation{amount: feeAmount, underlyingFees: underlyingFees}, nil
	}
	feeQuote, err := getFeeQuote(ctx, feeAmount, feeAssetID, params.AssetID)
	if err != nil {
		return feeEstimation{}, err
	}
	return feeEstimation{amount: mustBigInt(feeQuote.AmountIn), quote: &feeQuote, underlyingFees: underlyingFees}, nil
}

func (b *hotBridge) createWithdrawalIdentifier(params withdrawalParams, index int, txHash string) (withdrawalIdentifier, error) {
	info, err := b.mustAssetInfo(params.AssetID)
	if err != nil {
		return withdrawalIdentifier{}, err
	}
	return withdrawalIdentifier{landingChain: info.blockchain, index: index, withdrawalParams: params, txHash: txHash}, nil
}

// parseWithdrawalNonces (omni-sdk bridge-near): EXPERIMENTAL_tx_status receipts, "memo":"<nonce>" in each log.
func hotParseWithdrawalNonces(ctx context.Context, txHash string, sender string) ([]*big.Int, error) {
	var response struct {
		Result *struct {
			ReceiptsOutcome []struct {
				Outcome struct {
					Logs []string `json:"logs"`
				} `json:"outcome"`
			} `json:"receipts_outcome"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "dontcare",
		"method":  "EXPERIMENTAL_tx_status",
		"params":  map[string]string{"tx_hash": txHash, "sender_account_id": sender, "wait_until": "EXECUTED_OPTIMISTIC"},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, constants.NearRPCURL, strings.NewReader(string(payload)))
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
		return nil, nearRPCHTTPError{status: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	if (len(response.Error) > 0 && string(response.Error) != "null") || response.Result == nil {
		return nil, fmt.Errorf("NEAR RPC EXPERIMENTAL_tx_status failed: %s", string(response.Error))
	}

	nonces := []*big.Int{}
	for _, item := range response.Result.ReceiptsOutcome {
		for _, log := range item.Outcome.Logs {
			if match := hotMemoNonceRegex.FindStringSubmatch(log); match != nil {
				nonces = append(nonces, mustBigInt(match[1]))
			}
		}
	}
	return nonces, nil
}

func (b *hotBridge) getNonces(ctx context.Context, txHash string) ([]*big.Int, error) {
	// tx.accountId is the intents contract (IntentRelayerPublic)
	key := txHash + ":" + constants.NearIntentsContractID
	if cached, ok := b.noncesCache.get(key); ok {
		return cached, nil
	}
	rpcCtx, cancel := context.WithTimeout(ctx, hotNearRPCTimeout)
	defer cancel()
	nonces, err := hotParseWithdrawalNonces(rpcCtx, txHash, constants.NearIntentsContractID)
	if err != nil {
		return nil, err
	}
	b.noncesCache.set(key, nonces)
	return nonces, nil
}

func (b *hotBridge) describeWithdrawal(ctx context.Context, wid withdrawalIdentifier) (withdrawalStatus, error) {
	nonces, err := b.getNonces(ctx, wid.txHash)
	if err != nil {
		return withdrawalStatus{}, err
	}
	if wid.index >= len(nonces) || nonces[wid.index] == nil {
		return withdrawalStatus{}, fmt.Errorf("Withdrawal with given index is not found. TxHash: %s Index: %d", wid.txHash, wid.index)
	}
	nonce := nonces[wid.index]

	// getGaslessWithdrawStatus
	status, err := callFunction[json.RawMessage](ctx, hotRefuelContract, "get_withdrawal_hash", map[string]string{"nonce": nonce.String()})
	if err != nil {
		return withdrawalStatus{}, err
	}
	if status == nil {
		return withdrawalStatus{}, errors.New("NEAR RPC call get_withdrawal_hash returned an empty result")
	}
	// typeof status === "string": a JSON null (still pending) or any non-string value falls through to the API fallback
	var statusString string
	if strings.HasPrefix(strings.TrimSpace(string(*status)), `"`) && json.Unmarshal(*status, &statusString) == nil {
		if statusString == hotStatusCanceled {
			return withdrawalStatus{status: "failed", reason: "Withdrawal was cancelled"}, nil
		}
		return withdrawalStatus{status: "completed"}, nil
	}

	if hotFetchWithdrawalHashFromAPI(ctx, wid.txHash, nonce) {
		return withdrawalStatus{status: "completed"}, nil
	}
	return withdrawalStatus{status: "pending"}, nil
}

// fetchWithdrawalHashFromApi != null; any failure is false.
func hotFetchWithdrawalHashFromAPI(ctx context.Context, nearTxHash string, nonce *big.Int) bool {
	apiCtx, cancel := context.WithTimeout(ctx, hotAPIFallbackTimeout)
	defer cancel()

	// OmniApi.request: first endpoint with a 2xx response
	var resp *http.Response
	for _, endpoint := range hotAPIEndpoints {
		req, err := http.NewRequestWithContext(apiCtx, http.MethodGet, endpoint+"/api/v1/evm/bridge_withdrawal_hash?near_trx="+nearTxHash, nil)
		if err != nil {
			return false
		}
		req.Header.Set("omni-version", "v2")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Referer", "https://near-intents.org")
		req.Header.Set("Origin", "https://near-intents.org")
		r, err := api.Client.Do(req)
		if err != nil {
			continue
		}
		if r.StatusCode < 200 || r.StatusCode >= 300 {
			r.Body.Close()
			continue
		}
		resp = r
		break
	}
	if resp == nil {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return false
	}

	// HotApiWithdrawalResponseSchema: every field is required (hash may be null); any mismatch means null
	type hotAPIWithdrawal struct {
		Hash             *string  `json:"hash"`
		Nonce            *string  `json:"nonce"`
		NearTrx          *string  `json:"near_trx"`
		VerifiedWithdraw *bool    `json:"verified_withdraw"`
		ChainID          *float64 `json:"chain_id"`
	}
	var data struct {
		Hash        json.RawMessage     `json:"hash"`
		Nonce       *string             `json:"nonce"`
		NearTrx     *string             `json:"near_trx"`
		Withdrawals *[]hotAPIWithdrawal `json:"withdrawals"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}
	if data.Hash == nil || data.Nonce == nil || data.NearTrx == nil || data.Withdrawals == nil {
		return false
	}
	for _, withdrawal := range *data.Withdrawals {
		if withdrawal.Nonce == nil || withdrawal.NearTrx == nil || withdrawal.VerifiedWithdraw == nil || withdrawal.ChainID == nil {
			return false
		}
	}
	for _, withdrawal := range *data.Withdrawals {
		if *withdrawal.Nonce != nonce.String() {
			continue
		}
		return withdrawal.Hash != nil && isHex(strings.TrimPrefix(*withdrawal.Hash, "0x"))
	}
	return false
}
