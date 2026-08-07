package tests

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/providers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/signers"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const solanaMainnetUSDC = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

func liveSolanaPrivateKey(t *testing.T) string {
	t.Helper()
	pk := os.Getenv("HINKAL_SOLANA_PRIVATE_KEY")
	if pk == "" {
		t.Skip("set HINKAL_SOLANA_PRIVATE_KEY (base58) to run the Solana deposit test")
	}
	return pk
}

func solanaDepositToken(t *testing.T) (string, *big.Int) {
	t.Helper()
	token := os.Getenv("HINKAL_SOLANA_TOKEN")
	if token == "" {
		token = constants.SolanaNativeAddress // default: native SOL
	}
	amount := big.NewInt(1_000_000) // 0.001 SOL or 1 USDC unit; override below
	if v := os.Getenv("HINKAL_SOLANA_AMOUNT"); v != "" {
		parsed, ok := new(big.Int).SetString(v, 10)
		if !ok {
			t.Fatalf("bad HINKAL_SOLANA_AMOUNT: %q", v)
		}
		amount = parsed
	}
	return token, amount
}

func newLiveSolanaHinkal(t *testing.T, ctx context.Context) *hinkal.Hinkal {
	t.Helper()
	seed := os.Getenv("HINKAL_SEED_PHRASE")
	if seed == "" {
		t.Skip("set HINKAL_SEED_PHRASE to derive the shielded keys for the Solana deposit test")
	}
	signer, err := signers.NewPrivateKeySolanaSigner(liveSolanaPrivateKey(t))
	if err != nil {
		t.Fatalf("solana signer: %v", err)
	}
	pubKey, err := signer.GetPublicKey(ctx)
	if err != nil {
		t.Fatalf("solana pubkey: %v", err)
	}
	adapter, err := providers.NewSolanaProviderAdapter(constants.CurrentSolanaChainID, pubKey.String())
	if err != nil {
		t.Fatalf("solana adapter: %v", err)
	}
	adapter.InitConnector(signer)

	h := hinkal.NewHinkal(nil)
	if err := h.InitProviderAdapter(ctx, adapter); err != nil {
		t.Fatalf("init provider adapter: %v", err)
	}
	if err := h.InitUserKeysFromSeedPhrases(strings.Fields(seed)); err != nil {
		t.Fatalf("init user keys from seed phrase: %v", err)
	}
	if err := h.ResetMerkle(ctx, constants.CurrentSolanaChainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}
	return h
}

func solanaFreshFeeStructure(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, amountChanges []*big.Int) types.FeeStructure {
	t.Helper()
	nullifierCount := pretransaction.CalculateSolanaNullifierCount(ctx, h, chainID, []string{solanaMainnetUSDC}, amountChanges)
	solanaParams := &api.SolanaGasEstimateParams{MintTo: solanaMainnetUSDC, NullifierCount: nullifierCount}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, solanaMainnetUSDC, []string{solanaMainnetUSDC}, types.ExternalActionTransact, nil, nil, solanaParams)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	return feeStructure
}

func solanaPrivateBalance(t *testing.T, ctx context.Context, h *hinkal.Hinkal, tokenAddress string) *big.Int {
	t.Helper()
	if err := h.ResetMerkle(ctx, constants.CurrentSolanaChainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}
	balances, err := h.GetTotalBalance(ctx, constants.CurrentSolanaChainID, nil, "", true, false)
	if err != nil {
		t.Fatalf("get total balance: %v", err)
	}
	for _, b := range balances {
		if strings.EqualFold(b.Token.Erc20TokenAddress, tokenAddress) {
			return b.Balance
		}
	}
	return new(big.Int)
}
