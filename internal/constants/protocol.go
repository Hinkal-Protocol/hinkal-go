package constants

const (
	ZeroAddress         = "0x0000000000000000000000000000000000000000"
	SolanaNativeAddress = "11111111111111111111111111111111"

	// TronDefaultFeeLimitSun is the default fee limit (in SUN) for Tron contract calls.
	TronDefaultFeeLimitSun int64 = 1_000_000_000
	// TronFeePaddingBps is the buffer (basis points) added over the estimated Tron fee.
	TronFeePaddingBps int64 = 2_000

	EnclavePubkey = "0x566de53cee38c8e4e87636e137b206b4799ae79b87d563f416008946bdf14427"

	EnclaveSignerAddress = "0x655C3Aa937530E06A336458D1CF168d297B597A2"

	BpsDenominator int64 = 10_000

	MaxTronSelfOutputs = 4
)
