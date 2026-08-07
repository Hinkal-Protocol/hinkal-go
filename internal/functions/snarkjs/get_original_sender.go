package snarkjs

import "github.com/Hinkal-Protocol/hinkal-go/internal/constants"

func GetOriginalSender(externalAddress, relay string) string {
	if relay == constants.ZeroAddress {
		if externalAddress == "" {
			return constants.ZeroAddress
		}
		return externalAddress
	}
	return constants.ZeroAddress
}
