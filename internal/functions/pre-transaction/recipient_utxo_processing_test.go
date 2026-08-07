package pretransaction

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func zeroUtxoForToken(t *testing.T, tokenAddress string) *utxo.Utxo {
	t.Helper()
	u, err := utxo.NewUtxo(types.UtxoParams{Amount: big.NewInt(0), Erc20TokenAddress: tokenAddress, StealthAddress: "0x1", H0: &types.JubPoint{big.NewInt(1), big.NewInt(1)}, EncryptionKey: "0x" + "11"})
	if err != nil {
		t.Fatalf("build zero utxo: %v", err)
	}
	return u
}

func TestRecipientUtxoProcessing_AppendsToMatchingTokenAndPadsOthers(t *testing.T) {
	tokenA, tokenB := "0xaaa", "0xbbb"
	outputUtxosArray := [][]*utxo.Utxo{{zeroUtxoForToken(t, tokenA)}, {zeroUtxoForToken(t, tokenB)}}
	deltaChanges := []*big.Int{big.NewInt(-5), big.NewInt(0)}
	recipientInfo := bridging.PrivateRecipientInfo{
		RecipientInfo: "1,2,3,4,5,0x" + "11",
		Amount:        big.NewInt(7),
		Token:         types.ERC20Token{Erc20TokenAddress: tokenA},
	}

	if err := RecipientUtxoProcessing(recipientInfo, outputUtxosArray, deltaChanges, "1700000000"); err != nil {
		t.Fatalf("RecipientUtxoProcessing: %v", err)
	}

	if len(outputUtxosArray[0]) != 2 {
		t.Fatalf("expected token A output array to grow to 2, got %d", len(outputUtxosArray[0]))
	}
	if len(outputUtxosArray[1]) != 2 {
		t.Fatalf("expected token B output array padded to 2, got %d", len(outputUtxosArray[1]))
	}
	if deltaChanges[0].Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("expected deltaChanges[0] = -5+7 = 2, got %s", deltaChanges[0])
	}
	if deltaChanges[1].Sign() != 0 {
		t.Fatalf("expected deltaChanges[1] unchanged at 0, got %s", deltaChanges[1])
	}

	appended := outputUtxosArray[0][1]
	if !strings.EqualFold(appended.Erc20TokenAddress, tokenA) {
		t.Fatalf("expected appended recipient utxo token = %s, got %s", tokenA, appended.Erc20TokenAddress)
	}
	if appended.Amount.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("expected appended recipient utxo amount = 7, got %s", appended.Amount)
	}

	padded := outputUtxosArray[1][1]
	if !strings.EqualFold(padded.Erc20TokenAddress, tokenB) {
		t.Fatalf("expected padding utxo token = %s (own array's token), got %s", tokenB, padded.Erc20TokenAddress)
	}
	if padded.Amount.Sign() != 0 {
		t.Fatalf("expected padding utxo amount = 0, got %s", padded.Amount)
	}
}

func TestRecipientUtxoProcessing_NoMatchingToken(t *testing.T) {
	outputUtxosArray := [][]*utxo.Utxo{{zeroUtxoForToken(t, "0xaaa")}}
	deltaChanges := []*big.Int{big.NewInt(0)}
	recipientInfo := bridging.PrivateRecipientInfo{
		RecipientInfo: "1,2,3,4,5,0x11",
		Amount:        big.NewInt(1),
		Token:         types.ERC20Token{Erc20TokenAddress: "0xccc"},
	}
	if err := RecipientUtxoProcessing(recipientInfo, outputUtxosArray, deltaChanges, "1700000000"); err == nil {
		t.Fatalf("expected error when no output array contains the recipient's token")
	}
}
