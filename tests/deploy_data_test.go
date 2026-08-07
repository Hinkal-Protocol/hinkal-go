package tests

import (
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
)

func TestDeployDataAddressesLoad(t *testing.T) {
	addr, err := constants.HinkalAddress(constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Fatalf("HinkalAddress(EthMainnet): %v", err)
	}
	if addr == "" {
		t.Fatal("HinkalAddress(EthMainnet) is empty")
	}
}

func TestHinkalABILoadsPerChain(t *testing.T) {
	evm, err := contractabi.Hinkal(constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Fatalf("Hinkal(EthMainnet): %v", err)
	}
	evmTransact, ok := evm.Methods["transact"]
	if !ok {
		t.Fatal("EVM ABI missing transact method")
	}
	if len(evmTransact.Inputs) != 5 {
		t.Fatalf("EVM transact inputs = %d, want 5", len(evmTransact.Inputs))
	}

	tron, err := contractabi.Hinkal(constants.ChainIDs.TronMainnet)
	if err != nil {
		t.Fatalf("Hinkal(TronMainnet): %v", err)
	}
	tronTransact, ok := tron.Methods["transact"]
	if !ok {
		t.Fatal("Tron ABI missing transact method")
	}
	if len(tronTransact.Inputs) != 6 {
		t.Fatalf("Tron transact inputs = %d, want 6 (proofSignature + 5)", len(tronTransact.Inputs))
	}
	if tronTransact.Inputs[0].Name != "proofSignature" {
		t.Fatalf("Tron transact first input = %q, want proofSignature", tronTransact.Inputs[0].Name)
	}
}
