package constants

import (
	"fmt"
	"strconv"
)

// ServerConfig holds main-server route paths (subset of API_CONFIG.ROUTES in
// server.constants.ts that is actually served by the main server).
var ServerConfig = struct {
	CallLifiAPI                   string
	CallOkxAPI                    string
	CallNearIntentsQuote          string
	CallNearIntentsTokens         string
	GetOdosPriceForToken          func(chainID int, tokenAddress string) string
	Monitor                       string
	MonitorBatch                  string
	CheckRisk                     func(address string) string
	SimulateVolatileTokenTransfer string
	GetTemporaryWalletNonces      func(hashedEthereumAddress string, chainID int) string
	AddTemporaryWalletNonce       string
	RemoveTemporaryWalletNonce    string
	GetTokensInfo                 string
	GetTokensForChain             string
	GetTokenData                  func(tokenSymbol string) string
	RouteConfig                   string
	EvmRpcProxy                   func(chainID int, key string) string
	SolanaRpcProxy                func(key string) string
}{
	CallLifiAPI:           "/LifiBridgeData",
	CallOkxAPI:            "/OkxSwapData",
	CallNearIntentsQuote:  "/near-intents/quote",
	CallNearIntentsTokens: "/near-intents/tokens",
	GetOdosPriceForToken: func(chainID int, tokenAddress string) string {
		return fmt.Sprintf("/getOdosPriceForToken/%d/%s", chainID, tokenAddress)
	},
	Monitor:                       "/monitor",
	MonitorBatch:                  "/monitor/batch",
	CheckRisk:                     func(address string) string { return "/check-risk/" + address },
	SimulateVolatileTokenTransfer: "/simulate-volatile-token-transfer",
	GetTemporaryWalletNonces: func(hashedEthereumAddress string, chainID int) string {
		return fmt.Sprintf("/temporary-wallets/nonces/%s/%d", hashedEthereumAddress, chainID)
	},
	AddTemporaryWalletNonce:    "/temporary-wallets/add-nonce",
	RemoveTemporaryWalletNonce: "/temporary-wallets/remove-nonce",
	GetTokensInfo:              "/get-tokens-info",
	GetTokensForChain:          "/get-tokens-for-chain",
	GetTokenData:               func(tokenSymbol string) string { return "/get-token-data/" + tokenSymbol },
	RouteConfig:                "/rc",
	EvmRpcProxy:                func(chainID int, key string) string { return fmt.Sprintf("/evm/%d/%s", chainID, key) },
	SolanaRpcProxy:             func(key string) string { return "/solana/" + key },
}

// SnapshotServerConfig holds snapshot-server route paths. In server.constants.ts
// these live under API_CONFIG.ROUTES but are fetched against the snapshot server.
var SnapshotServerConfig = struct {
	GetSnapshots       func(chainID int) string
	MerkleTreeSiblings string
	Events             string
}{
	GetSnapshots:       func(chainID int) string { return fmt.Sprintf("/snapshots/%d", chainID) },
	MerkleTreeSiblings: "/merkle-tree-siblings",
	Events:             "/events",
}

// RelayerConfig mirrors RELAYER_CONFIG from server.constants.ts.
var RelayerConfig = struct {
	GetGasEstimate                           func(tokenAddress, externalActionID string, erc20TokenAddressLength int, gasAmount *int) string
	GetTokenPrices                           string
	Transact                                 string
	TransactBatch                            string
	SolanaTransact                           string
	SolanaTransactBatch                      string
	GetIdleRelay                             string
	EmitTxPublicData                         string
	EmitReferralTx                           string
	GetScheduledTransactions                 string
	GetScheduledTransactionByID              func(scheduleID string) string
	GetScheduledTransactionsNullifierIndexes string
	GetTokenPriceChartData                   string
	GetTokenPreviousDayPrices                string
	GetPrivateTransactionsRatio              func(hashedOwner string) string
	SavePrivacyScoreAndGetPrivacyRank        string
}{
	GetGasEstimate: func(tokenAddress, externalActionID string, erc20TokenAddressLength int, gasAmount *int) string {
		gas := "undefined"
		if gasAmount != nil {
			gas = strconv.Itoa(*gasAmount)
		}
		return fmt.Sprintf("/gas-estimation/%s/%d/%s/%s", tokenAddress, erc20TokenAddressLength, externalActionID, gas)
	},
	GetTokenPrices:                           "/get-token-prices",
	Transact:                                 "/general-transact",
	TransactBatch:                            "/general-transact-batch",
	SolanaTransact:                           "/solana-transact",
	SolanaTransactBatch:                      "/solana-transact-batch",
	GetIdleRelay:                             "/get-idle-relay",
	EmitTxPublicData:                         "/emit-tx-public-data",
	EmitReferralTx:                           "/emit-referral-tx",
	GetScheduledTransactions:                 "/scheduled-transactions",
	GetScheduledTransactionByID:              func(scheduleID string) string { return "/scheduled-transactions/" + scheduleID },
	GetScheduledTransactionsNullifierIndexes: "/scheduled-transactions/nullifier-indexes",
	GetTokenPriceChartData:                   "/get-token-price-chart-data",
	GetTokenPreviousDayPrices:                "/get-token-previous-day-prices",
	GetPrivateTransactionsRatio:              func(hashedOwner string) string { return "/get-private-transactions-ratio/" + hashedOwner },
	SavePrivacyScoreAndGetPrivacyRank:        "/save-privacy-score-and-get-privacy-rank",
}

// EnclaveConfig holds enclave-server route paths (inlined in TS enclave calls).
var EnclaveConfig = struct {
	Handshake                  string
	DecryptUtxos               string
	GenerateProofs             string
	StoreClaimableKey          string
	GetUtxos                   string
	StoreAndGetSignature       string
	SignProof                  string
	PrepareTx                  string
	PrepareSolanaTx            string
	PrepareStuckWithdraw       string
	PrepareSolanaStuckWithdraw string
	FinalizeTx                 string
	FinalizeSolanaTx           string
	FinalizeTxRelay            string
	FinalizeSolanaTxRelay      string
	GetBalances                string
}{
	Handshake:                  "/handshake",
	DecryptUtxos:               "/decrypt-utxos",
	GenerateProofs:             "/generate-proofs",
	StoreClaimableKey:          "/store-claimable-key",
	GetUtxos:                   "/get-user-utxos",
	StoreAndGetSignature:       "/store-and-get-user-signature",
	SignProof:                  "/sign-proof",
	GetBalances:                "/get-user-balances",
	PrepareTx:                  "/prepare-tx",
	PrepareSolanaTx:            "/prepare-solana-tx",
	PrepareStuckWithdraw:       "/prepare-stuck-withdraw",
	PrepareSolanaStuckWithdraw: "/prepare-solana-stuck-withdraw",
	FinalizeTx:                 "/finalize-tx",
	FinalizeSolanaTx:           "/finalize-solana-tx",
	FinalizeTxRelay:            "/finalize-tx-relay",
	FinalizeSolanaTxRelay:      "/finalize-solana-tx-relay",
}
