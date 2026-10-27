package solanautils

import (
	"context"
	"log"
	"math"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	maxComputeUnitLimit            = 1_400_000
	minComputeUnitLimit            = 200_000
	computeUnitPaddingPercent      = 0.2
	maxLoadedAccountsDataSizeBytes = 64 * 1024 * 1024
)

func buildV1Transaction(
	instructions []solana.Instruction,
	blockhash solana.Hash,
	payer solana.PublicKey,
	computeUnitLimit uint32,
) (*solana.Transaction, error) {
	config := solana.TransactionConfig{}.
		WithComputeUnitLimit(computeUnitLimit).
		WithLoadedAccountsDataSizeLimit(maxLoadedAccountsDataSizeBytes)
	return solana.NewTransaction(instructions, blockhash, solana.TransactionPayer(payer), solana.TransactionV1Config(config))
}

// Simulate at the ceiling, then reserve what was actually consumed plus padding. Any failure
// falls back to the ceiling, which is what the transaction would have asked for anyway.
func estimateV1ComputeUnitLimit(
	ctx context.Context,
	connection *rpc.Client,
	payer solana.PublicKey,
	instructions []solana.Instruction,
	blockhash solana.Hash,
) uint32 {
	tx, err := buildV1Transaction(instructions, blockhash, payer, maxComputeUnitLimit)
	if err != nil {
		return maxComputeUnitLimit
	}
	simulation, err := connection.SimulateTransactionWithOpts(ctx, tx, &rpc.SimulateTransactionOpts{
		ReplaceRecentBlockhash: true,
		Commitment:             rpc.CommitmentConfirmed,
	})
	if err != nil || simulation == nil || simulation.Value == nil {
		return maxComputeUnitLimit
	}
	if simulation.Value.Err != nil {
		log.Printf("solana compute-unit simulation failed: %v", simulation.Value.Err)
	}
	if simulation.Value.UnitsConsumed == nil || *simulation.Value.UnitsConsumed == 0 {
		return maxComputeUnitLimit
	}
	padded := uint64(math.Ceil(float64(*simulation.Value.UnitsConsumed) * (1 + computeUnitPaddingPercent)))
	return uint32(max(minComputeUnitLimit, min(padded, maxComputeUnitLimit)))
}

func NewV1Transaction(
	ctx context.Context,
	connection *rpc.Client,
	signer solana.PublicKey,
	instructions []solana.Instruction,
	blockhash solana.Hash,
) (*solana.Transaction, error) {
	computeUnitLimit := estimateV1ComputeUnitLimit(ctx, connection, signer, instructions, blockhash)
	return buildV1Transaction(instructions, blockhash, signer, computeUnitLimit)
}
