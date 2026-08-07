package snarkjs

import (
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

const ProverVersion = "v4"

var localVerifiers = map[string]string{
	"mainEVMCircuit1x2x1v4Wasm": "mainEVMCircuit1x2x1-4.wasm",
	"mainEVMCircuit1x2x1v4Zkey": "mainEVMCircuit1x2x1_final-4.zkey",
	"mainEVMCircuit1x6x1v4Wasm": "mainEVMCircuit1x6x1-4.wasm",
	"mainEVMCircuit1x6x1v4Zkey": "mainEVMCircuit1x6x1_final-4.zkey",
	"mainEVMCircuit2x2x1v4Wasm": "mainEVMCircuit2x2x1-4.wasm",
	"mainEVMCircuit2x2x1v4Zkey": "mainEVMCircuit2x2x1_final-4.zkey",
	"mainEVMCircuit2x6x1v4Wasm": "mainEVMCircuit2x6x1-4.wasm",
	"mainEVMCircuit2x6x1v4Zkey": "mainEVMCircuit2x6x1_final-4.zkey",
	"mainEVMCircuit3x2x1v4Wasm": "mainEVMCircuit3x2x1-4.wasm",
	"mainEVMCircuit3x2x1v4Zkey": "mainEVMCircuit3x2x1_final-4.zkey",
	"mainEVMCircuit3x6x1v4Wasm": "mainEVMCircuit3x6x1-4.wasm",
	"mainEVMCircuit3x6x1v4Zkey": "mainEVMCircuit3x6x1_final-4.zkey",
	"mainEVMCircuit4x2x1v4Wasm": "mainEVMCircuit4x2x1-4.wasm",
	"mainEVMCircuit4x2x1v4Zkey": "mainEVMCircuit4x2x1_final-4.zkey",
	"mainEVMCircuit4x6x1v4Wasm": "mainEVMCircuit4x6x1-4.wasm",
	"mainEVMCircuit4x6x1v4Zkey": "mainEVMCircuit4x6x1_final-4.zkey",
	"mainEVMCircuit5x2x1v4Wasm": "mainEVMCircuit5x2x1-4.wasm",
	"mainEVMCircuit5x2x1v4Zkey": "mainEVMCircuit5x2x1_final-4.zkey",
	"mainEVMCircuit5x6x1v4Wasm": "mainEVMCircuit5x6x1-4.wasm",
	"mainEVMCircuit5x6x1v4Zkey": "mainEVMCircuit5x6x1_final-4.zkey",
	"mainEVMCircuit1x2x2v4Wasm": "mainEVMCircuit1x2x2-4.wasm",
	"mainEVMCircuit1x2x2v4Zkey": "mainEVMCircuit1x2x2_final-4.zkey",
	"mainEVMCircuit1x6x2v4Wasm": "mainEVMCircuit1x6x2-4.wasm",
	"mainEVMCircuit1x6x2v4Zkey": "mainEVMCircuit1x6x2_final-4.zkey",
	"mainEVMCircuit2x2x2v4Wasm": "mainEVMCircuit2x2x2-4.wasm",
	"mainEVMCircuit2x2x2v4Zkey": "mainEVMCircuit2x2x2_final-4.zkey",
	"mainEVMCircuit2x6x2v4Wasm": "mainEVMCircuit2x6x2-4.wasm",
	"mainEVMCircuit2x6x2v4Zkey": "mainEVMCircuit2x6x2_final-4.zkey",
	"mainEVMCircuitMin0v4Wasm":  "mainEVMCircuitMin0-4.wasm",
	"mainEVMCircuitMin0v4Zkey":  "mainEVMCircuitMin0_final-4.zkey",

	"mainSolanaCircuit1x2x1v4Zkey": "mainSolanaCircuit1x2x1_final-4.zkey",
	"mainSolanaCircuit1x2x1v4Wasm": "mainSolanaCircuit1x2x1-4.wasm",
	"mainSolanaCircuit1x2x2v4Zkey": "mainSolanaCircuit1x2x2_final-4.zkey",
	"mainSolanaCircuit1x2x2v4Wasm": "mainSolanaCircuit1x2x2-4.wasm",
	"mainSolanaCircuit1x6x1v4Zkey": "mainSolanaCircuit1x6x1_final-4.zkey",
	"mainSolanaCircuit1x6x1v4Wasm": "mainSolanaCircuit1x6x1-4.wasm",
	"mainSolanaCircuit1x6x2v4Zkey": "mainSolanaCircuit1x6x2_final-4.zkey",
	"mainSolanaCircuit1x6x2v4Wasm": "mainSolanaCircuit1x6x2-4.wasm",
	"mainSolanaCircuit2x2x1v4Zkey": "mainSolanaCircuit2x2x1_final-4.zkey",
	"mainSolanaCircuit2x2x1v4Wasm": "mainSolanaCircuit2x2x1-4.wasm",
	"mainSolanaCircuit2x6x1v4Zkey": "mainSolanaCircuit2x6x1_final-4.zkey",
	"mainSolanaCircuit2x6x1v4Wasm": "mainSolanaCircuit2x6x1-4.wasm",

	"commitmentCalculator1x2v4Wasm": "commitmentCalculator1x2-4.wasm",
	"commitmentCalculator1x2v4Zkey": "commitmentCalculator1x2_final-4.zkey",
	"commitmentCalculator1x2v4VK":   "commitmentCalculator1x2_final-4_verification_key.json",
	"commitmentCalculator1x6v4Wasm": "commitmentCalculator1x6-4.wasm",
	"commitmentCalculator1x6v4Zkey": "commitmentCalculator1x6_final-4.zkey",
	"commitmentCalculator1x6v4VK":   "commitmentCalculator1x6_final-4_verification_key.json",
	"commitmentCalculator2x2v4Wasm": "commitmentCalculator2x2-4.wasm",
	"commitmentCalculator2x2v4Zkey": "commitmentCalculator2x2_final-4.zkey",
	"commitmentCalculator2x2v4VK":   "commitmentCalculator2x2_final-4_verification_key.json",
	"commitmentCalculator2x6v4Wasm": "commitmentCalculator2x6-4.wasm",
	"commitmentCalculator2x6v4Zkey": "commitmentCalculator2x6_final-4.zkey",
	"commitmentCalculator2x6v4VK":   "commitmentCalculator2x6_final-4_verification_key.json",
	"commitmentCalculator3x2v4Wasm": "commitmentCalculator3x2-4.wasm",
	"commitmentCalculator3x2v4Zkey": "commitmentCalculator3x2_final-4.zkey",
	"commitmentCalculator3x2v4VK":   "commitmentCalculator3x2_final-4_verification_key.json",
	"commitmentCalculator3x6v4Wasm": "commitmentCalculator3x6-4.wasm",
	"commitmentCalculator3x6v4Zkey": "commitmentCalculator3x6_final-4.zkey",
	"commitmentCalculator3x6v4VK":   "commitmentCalculator3x6_final-4_verification_key.json",
	"commitmentCalculator4x2v4Wasm": "commitmentCalculator4x2-4.wasm",
	"commitmentCalculator4x2v4Zkey": "commitmentCalculator4x2_final-4.zkey",
	"commitmentCalculator4x2v4VK":   "commitmentCalculator4x2_final-4_verification_key.json",
	"commitmentCalculator4x6v4Wasm": "commitmentCalculator4x6-4.wasm",
	"commitmentCalculator4x6v4Zkey": "commitmentCalculator4x6_final-4.zkey",
	"commitmentCalculator4x6v4VK":   "commitmentCalculator4x6_final-4_verification_key.json",
	"commitmentCalculator5x2v4Wasm": "commitmentCalculator5x2-4.wasm",
	"commitmentCalculator5x2v4Zkey": "commitmentCalculator5x2_final-4.zkey",
	"commitmentCalculator5x2v4VK":   "commitmentCalculator5x2_final-4_verification_key.json",
	"commitmentCalculator5x6v4Wasm": "commitmentCalculator5x6-4.wasm",
	"commitmentCalculator5x6v4Zkey": "commitmentCalculator5x6_final-4.zkey",
	"commitmentCalculator5x6v4VK":   "commitmentCalculator5x6_final-4_verification_key.json",
}

var (
	prodVerifiersOnce sync.Once
	prodVerifiers     map[string]string
)

func getProdVerifiers() map[string]string {
	prodVerifiersOnce.Do(func() {
		gateway := constants.GetBackEndURL() + "/verifiers-v4/"
		prodVerifiers = make(map[string]string, len(localVerifiers))
		for key, name := range localVerifiers {
			prodVerifiers[key] = gateway + name
		}
	})
	return prodVerifiers
}

func isLocalProverChain(chainID int) bool {
	return chainID == constants.ChainIDs.Localhost ||
		chainID == constants.ChainIDs.SolanaLocalnet ||
		chainID == constants.ChainIDs.TronLocalnet
}

func GetWASMFile(filename string, chainID int) string {
	key := filename + ProverVersion + "Wasm"
	if isLocalProverChain(chainID) {
		return localVerifiers[key]
	}
	return getProdVerifiers()[key]
}

func GetZKeyFile(filename string, chainID int) string {
	key := filename + ProverVersion + "Zkey"
	if isLocalProverChain(chainID) {
		return localVerifiers[key]
	}
	return getProdVerifiers()[key]
}

func GetVKFile(filename string, chainID int) string {
	key := filename + ProverVersion + "VK"
	if isLocalProverChain(chainID) {
		return localVerifiers[key]
	}
	return getProdVerifiers()[key]
}
