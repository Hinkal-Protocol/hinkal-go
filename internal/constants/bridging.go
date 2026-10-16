package constants

import "time"

const (
	DefaultBridgingSlippage        = 0.7
	DefaultBridgingSlippageDecimal = DefaultBridgingSlippage * 0.01
	BridgeArrivalTimeout           = 3 * time.Minute
	BridgeArrivalPollInterval      = 5 * time.Second
	SlippageScalingFactor          = 1_000_000
	LowAmountBridgingSlippage      = 1.5
	LowAmountUSDThreshold          = 10
	// A bridge parks funds on its temporary wallet until the shield lands, so a wallet this young holds
	// funds in transit rather than stuck ones.
	TemporaryWalletInTransitMS = 3 * 60 * 1000 // 3 min
)
