package constants

import "strings"

const (
	ZeroAddress         = "0x0000000000000000000000000000000000000000"
	SolanaNativeAddress = "11111111111111111111111111111111"

	// ArcTestnetUSDCAddress is the address Arc testnet's native gas token is represented as, to
	// avoid it showing up as a second, duplicate entry next to the "real" USDC at zeroAddress.
	ArcTestnetUSDCAddress = "0x3600000000000000000000000000000000000000"

	// TronDefaultFeeLimitSun is the default fee limit (in SUN) for Tron contract calls.
	TronDefaultFeeLimitSun int64 = 1_000_000_000
	// TronFeePaddingBps is the buffer (basis points) added over the estimated Tron fee.
	TronFeePaddingBps int64 = 2_000

	EnclavePubkey = "0x566de53cee38c8e4e87636e137b206b4799ae79b87d563f416008946bdf14427"

	EnclaveSignerAddress = "0x655C3Aa937530E06A336458D1CF168d297B597A2"

	BpsDenominator int64 = 10_000

	MaxTronSelfOutputs = 4

	// ReceiveVaultSettleRequestTTLSeconds bounds an account proof handed to the main server.
	ReceiveVaultSettleRequestTTLSeconds = 5 * 60
	// ReceiveVaultRecoveryTTLSeconds bounds a recovery signature handed to the factory.
	ReceiveVaultRecoveryTTLSeconds = 15 * 60
	// ReceiveVaultVariableRate is the settle fee rate, in basis points.
	ReceiveVaultVariableRate int64 = 25
)

func IsNativePlaceholderAddress(address string, chainID ...int) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}
	if len(chainID) == 0 {
		return strings.EqualFold(address, ZeroAddress) || strings.EqualFold(address, SolanaNativeAddress)
	}
	if IsSolanaLike(chainID[0]) {
		return strings.EqualFold(address, SolanaNativeAddress)
	}
	return strings.EqualFold(address, ZeroAddress)
}
