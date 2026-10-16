package constants

import "regexp"

var (
	tronPrefixedHexPattern = regexp.MustCompile(`^41[0-9a-fA-F]{40}$`)
	evmHexPattern          = regexp.MustCompile(`^0[xX][0-9a-fA-F]{40}$`)
)

func IsTronHexAddress(addr string) bool {
	return evmHexPattern.MatchString(addr) || tronPrefixedHexPattern.MatchString(addr)
}
