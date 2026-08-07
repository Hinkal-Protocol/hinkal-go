package constants

import "time"

const (
	DefaultBridgingSlippageDecimal = 0.007
	BridgeArrivalTimeout           = 3 * time.Minute
	BridgeArrivalPollInterval      = 5 * time.Second
	SlippageScalingFactor          = 1_000_000
)
