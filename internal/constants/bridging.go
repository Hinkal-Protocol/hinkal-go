package constants

const (
	DefaultBridgingSlippage        = 0.7
	DefaultBridgingSlippageDecimal = DefaultBridgingSlippage * 0.01
	LowAmountBridgingSlippage      = 1.5
	LowAmountUSDThreshold          = 10
	// A bridge parks funds on its temporary wallet while the route executes, so a wallet this young holds
	// funds in transit rather than stuck ones.
	TemporaryWalletInTransitMS = 3 * 60 * 1000 // 3 min
	BridgeVaultMinTTLSeconds   = 30 * 60       // 30 min
)
