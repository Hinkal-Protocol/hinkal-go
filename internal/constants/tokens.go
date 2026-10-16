package constants

import "strings"

// nonStandardStorageTokens mirrors NON_STANDARD_STORAGE_TOKENS (tokens.constants.ts): tokens without known
// balance/allowance storage offsets that are still safe to move, so they are not treated as volatile.
var nonStandardStorageTokens = map[string]struct{}{
	"0x23d7cffd0876f3abb6b074287ba2aeefbc83825d": {}, // QCAD
	"0xd2a530170d71a9cfe1651fb468e2b98f7ed7456b": {}, // AUDF
	"0xcec185eb182c47d1ba1efc84e6959e18cd620be4": {}, // cbBTC (Robinhood)
	"0x2dc99af320bc317c567f24ee95811dcbd5983dfd": {}, // wstETH (Robinhood)
	"0x322f0929c4625ed5bad873c95208d54e1c003b2d": {}, // TSLA (Robinhood)
	"0xaf3d76f1834a1d425780943c99ea8a608f8a93f9": {}, // AAPL (Robinhood)
	"0xd0601ce157db5bdc3162bbac2a2c8af5320d9eec": {}, // NVDA (Robinhood)
	"0x12f190a9f9d7d37a250758b26824b97ce941bf54": {}, // AMZN (Robinhood)
	"0xe93237c50d904957cf27e7b1133b510c669c2e74": {}, // MSFT (Robinhood)
	"0x2e0847e8910a9732eb3fb1bb4b70a580adad4fe3": {}, // GOOGL (Robinhood)
	"0xc0d6457c16cc70d6790dd43521c899c87ce02f35": {}, // META (Robinhood)
	"0xec262a75e413fafd0df80480274532c79d42da09": {}, // MSTR (Robinhood)
	"0x117cc2133c37b721f49de2a7a74833232b3b4c0c": {}, // SPY (Robinhood)
	"0x0f17206447090e464c277571124dd2688e48aea9": {}, // QCOM (Robinhood)
	"0x0dc4f92879b7670e5f4e4e6e3c801d229129d90d": {}, // WARS (Arc Mainnet)
	"0xd76f5faf6888e24d9f04bf92a0c8b921fe4390e0": {}, // WBRL (Arc Mainnet)
	"0x337e7456b420bd3481e7fa61fa9850343d610d34": {}, // WMXN (Arc Mainnet)
	"0x8a1d45e102e886510e891d2ec656a708991e2d76": {}, // WCOP (Arc Mainnet)
	"0x4f34c8b3b5fb6d98da888f0fea543d4d9c9f2ebe": {}, // WPEN (Arc Mainnet)
	"0x61d450a098b6a7f69fc4b98ce68198fe59768651": {}, // WCLP (Arc Mainnet)
}

func IsNonStandardStorageToken(address string) bool {
	_, ok := nonStandardStorageTokens[strings.ToLower(address)]
	return ok
}
