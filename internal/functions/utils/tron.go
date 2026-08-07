package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mr-tron/base58"
)

const tronAddressPrefix byte = 0x41

func EVMHexToTronBase58Address(addr string) (string, error) {
	if strings.HasPrefix(addr, "T") {
		return addr, nil
	}

	payload, err := tronPayloadFromHex(addr)
	if err != nil {
		return "", err
	}
	return encodeTronBase58Check(payload), nil
}

func ToTronBase58IfHex(addr string) (string, error) {
	if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
		return EVMHexToTronBase58Address(addr)
	}
	return addr, nil
}

func AddressToHexFormat(addr string) (string, error) {
	if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
		return addr, nil
	}
	if strings.HasPrefix(addr, "41") {
		payload, err := tronPayloadFromPrefixedHex(addr)
		if err != nil {
			return "", err
		}
		return "0x" + hex.EncodeToString(payload[1:]), nil
	}
	if strings.HasPrefix(addr, "T") {
		payload, err := decodeTronBase58Check(addr)
		if err != nil {
			return "", err
		}
		return "0x" + hex.EncodeToString(payload[1:]), nil
	}

	return "", errors.New("failed to convert address to hex format")
}

func NormalizeTronAddr(addr string) (string, error) {
	if strings.HasPrefix(addr, "T") {
		return AddressToHexFormat(addr)
	}
	return addr, nil
}

func tronPayloadFromPrefixedHex(addr string) ([]byte, error) {
	bytes, err := hex.DecodeString(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid Tron hex address: %w", err)
	}
	if len(bytes) != 21 {
		return nil, fmt.Errorf("invalid Tron prefixed hex address length %d", len(bytes))
	}
	if bytes[0] != tronAddressPrefix {
		return nil, errors.New("invalid Tron hex address prefix")
	}
	return bytes, nil
}

func tronPayloadFromHex(addr string) ([]byte, error) {
	hexAddr := strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "0X")
	bytes, err := hex.DecodeString(hexAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid Tron hex address: %w", err)
	}

	switch len(bytes) {
	case 20:
		payload := make([]byte, 21)
		payload[0] = tronAddressPrefix
		copy(payload[1:], bytes)
		return payload, nil
	case 21:
		if bytes[0] != tronAddressPrefix {
			return nil, errors.New("invalid Tron hex address prefix")
		}
		return bytes, nil
	default:
		return nil, fmt.Errorf("invalid Tron hex address length %d", len(bytes))
	}
}

func encodeTronBase58Check(payload []byte) string {
	withChecksum := make([]byte, 0, len(payload)+4)
	withChecksum = append(withChecksum, payload...)
	withChecksum = append(withChecksum, tronChecksum(payload)...)
	return base58.Encode(withChecksum)
}

func decodeTronBase58Check(addr string) ([]byte, error) {
	decoded, err := base58.Decode(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid Tron base58 address: %w", err)
	}
	if len(decoded) != 25 {
		return nil, fmt.Errorf("invalid Tron base58 address length %d", len(decoded))
	}

	payload := decoded[:21]
	if payload[0] != tronAddressPrefix {
		return nil, errors.New("invalid Tron base58 address prefix")
	}
	if !sameBytes(decoded[21:], tronChecksum(payload)) {
		return nil, errors.New("invalid Tron base58 address checksum")
	}
	return payload, nil
}

func tronChecksum(payload []byte) []byte {
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	return second[:4]
}

func sameBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
