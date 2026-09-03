package constants

import "math/big"

const DefaultFeeToken = ZeroAddress

var (
	ExtendedNonNativeGasCostTokenSymbolOptions = []string{"USDC", "USDT", "DAI"}
	TempoGasCostTokenSymbolOptions             = []string{"pathUSD", "USDT0", "USDC.e"}
)

func NonNativeGasCostTokenSymbolOptions(chainID int) []string {
	if IsTronLike(chainID) {
		return []string{"USDT"}
	}
	if IsTempo(chainID) {
		return TempoGasCostTokenSymbolOptions
	}
	return ExtendedNonNativeGasCostTokenSymbolOptions
}

func TwentyPercentBPS() *big.Int              { return big.NewInt(2000) }
func BPSDenominator() *big.Int                { return big.NewInt(10000) }
func PaySendVariableRate() *big.Int           { return big.NewInt(25) }
func HinkalPrivateSendVariableRate() *big.Int { return big.NewInt(25) }
func HinkalSwapVariableRate() *big.Int        { return big.NewInt(35) }

const (
	TempoDefaultGasTokenAddress = "0x20c0000000000000000000000000000000000000"
	TempoUsdcEAddress           = "0x20c000000000000000000000b9537d11c60e8b50"
	TempoUsdt0Address           = "0x20c00000000000000000000014f22ca97301eb73"
)
