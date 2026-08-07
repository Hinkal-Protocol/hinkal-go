package tests

import (
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func TestTronAddressConversions(t *testing.T) {
	hexAddr := "0xa614f803B6FD780986A42c78Ec9c7f77e6DeD13C"
	tronAddr := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	tronHexAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"

	gotTron, err := utils.EVMHexToTronBase58Address(hexAddr)
	if err != nil {
		t.Fatalf("evmHexToTronBase58Address: %v", err)
	}
	assertEqual(t, "tron base58", gotTron, tronAddr)

	gotHex, err := utils.AddressToHexFormat(tronAddr)
	if err != nil {
		t.Fatalf("addressToHexFormat(base58): %v", err)
	}
	if !strings.EqualFold(gotHex, hexAddr) {
		t.Fatalf("addressToHexFormat(base58) = %s, want %s", gotHex, hexAddr)
	}

	gotHex, err = utils.AddressToHexFormat(tronHexAddr)
	if err != nil {
		t.Fatalf("addressToHexFormat(41hex): %v", err)
	}
	if !strings.EqualFold(gotHex, hexAddr) {
		t.Fatalf("addressToHexFormat(41hex) = %s, want %s", gotHex, hexAddr)
	}

	gotNormalized, err := utils.NormalizeTronAddr(tronAddr)
	if err != nil {
		t.Fatalf("normalizeTronAddr: %v", err)
	}
	if !strings.EqualFold(gotNormalized, hexAddr) {
		t.Fatalf("normalizeTronAddr = %s, want %s", gotNormalized, hexAddr)
	}
}
