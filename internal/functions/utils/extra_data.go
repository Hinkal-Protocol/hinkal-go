package utils

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// extraData layout (mirrors Constants.sol): [0, 36) orderId slot (UTF-8 UUID, zero-padded), [36] version.
// This SDK always writes V1: signedMessageHash binds stealthAddress and calldataHash skips Emporium's v, r, s.
const (
	ExtraDataVersionOffset = 36
	ExtraDataV1            = 1
)

// EncodeExtraData builds V1 extraData carrying the (optional) orderId.
func EncodeExtraData(orderID string) (string, error) {
	raw := []byte(orderID)
	if len(raw) > ExtraDataVersionOffset {
		return "", fmt.Errorf("orderId does not fit the %d-byte extraData slot", ExtraDataVersionOffset)
	}
	out := make([]byte, ExtraDataVersionOffset+1)
	copy(out, raw)
	out[ExtraDataVersionOffset] = ExtraDataV1
	return hexutil.Encode(out), nil
}
