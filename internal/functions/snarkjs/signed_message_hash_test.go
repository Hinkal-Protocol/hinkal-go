package snarkjs_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
)

func evmParams(message int64) snarkjs.EvmSignedMessageHashParams {
	return snarkjs.EvmSignedMessageHashParams{
		RootHashHinkal:      big.NewInt(1),
		Erc20TokenAddresses: []string{"0x0000000000000000000000000000000000000001"},
		AmountChanges:       []*big.Int{big.NewInt(100)},
		OutTimeStamp:        big.NewInt(1700000000),
		InNullifiers:        [][]string{{"111"}},
		OutCommitments:      [][]string{{"222"}},
		CalldataHash:        big.NewInt(333),
		Message:             big.NewInt(message),
		OutH1Ax:             big.NewInt(4),
		OutH1Ay:             big.NewInt(5),
		H0Ax:                big.NewInt(6),
		H0Ay:                big.NewInt(7),
	}
}

func solanaParams(message int64) snarkjs.SolanaSignedMessageHashParams {
	return snarkjs.SolanaSignedMessageHashParams{
		RootHashHinkal:               big.NewInt(1),
		MintAccountPart1:             []*big.Int{big.NewInt(11)},
		MintAccountPart2:             []*big.Int{big.NewInt(12)},
		AmountChanges:                []*big.Int{big.NewInt(100)},
		OutTimeStamp:                 big.NewInt(1700000000),
		InNullifiers:                 [][]string{{"111"}},
		OutCommitments:               [][]string{{"222"}},
		CalldataHash:                 big.NewInt(333),
		Message:                      big.NewInt(message),
		SwapperAccountAdditionalSeed: big.NewInt(9),
		OutH1Ay:                      big.NewInt(5),
		H0Ax:                         big.NewInt(6),
		H0Ay:                         big.NewInt(7),
	}
}

func TestComputeSignedMessageHashEvm(t *testing.T) {
	const want = "14071578731421924333066389602483315169090043211846388287451126650864640202556"
	got, err := snarkjs.ComputeSignedMessageHashEvm(evmParams(42))
	if err != nil {
		t.Fatalf("ComputeSignedMessageHashEvm: %v", err)
	}
	if got.String() != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}

	other, err := snarkjs.ComputeSignedMessageHashEvm(evmParams(43))
	if err != nil {
		t.Fatalf("ComputeSignedMessageHashEvm: %v", err)
	}
	if other.Cmp(got) == 0 {
		t.Fatal("changing the message did not change the signed hash")
	}
}

func TestComputeSignedMessageHashEvm_InvalidTokenAddress(t *testing.T) {
	_, err := snarkjs.ComputeSignedMessageHashEvm(snarkjs.EvmSignedMessageHashParams{
		Erc20TokenAddresses: []string{"not-a-number"},
	})
	if err == nil || !strings.Contains(err.Error(), "not-a-number") {
		t.Fatalf("err = %v, want it to mention the unparseable address", err)
	}
}

func TestComputeSignedMessageHashSolana(t *testing.T) {
	const want = "15840274138361009823111928560195586450878935874587583816412673260180308003238"
	got, err := snarkjs.ComputeSignedMessageHashSolana(solanaParams(42))
	if err != nil {
		t.Fatalf("ComputeSignedMessageHashSolana: %v", err)
	}
	if got.String() != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}

	other, err := snarkjs.ComputeSignedMessageHashSolana(solanaParams(43))
	if err != nil {
		t.Fatalf("ComputeSignedMessageHashSolana: %v", err)
	}
	if other.Cmp(got) == 0 {
		t.Fatal("changing the message did not change the signed hash")
	}
}
