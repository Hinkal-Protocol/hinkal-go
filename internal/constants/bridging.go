package constants

import "time"

const (
	DefaultBridgingSlippage        = 0.7
	DefaultBridgingSlippageDecimal = DefaultBridgingSlippage * 0.01
	BridgeArrivalTimeout           = 3 * time.Minute
	BridgeArrivalPollInterval      = 5 * time.Second
	SlippageScalingFactor          = 1_000_000
)
