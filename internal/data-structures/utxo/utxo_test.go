package utxo_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func testSignature() string {
	return "0x" + strings.Repeat("ab", 65)
}

func TestNewStyleUtxoDerivesNonIdentityH0(t *testing.T) {
	uk := cryptokeys.NewUserKeys(testSignature())
	nullifyingKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	spendingKeyPair, err := uk.GetSpendingKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	u, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: constants.ZeroAddress,
		NullifyingKey:     nullifyingKey,
		SpendingPublicKey: []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]},
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.H0 == nil {
		t.Fatal("missing H0")
	}
	if (*u.H0)[0].Sign() == 0 && (*u.H0)[1].Cmp(big.NewInt(1)) == 0 {
		t.Fatal("derived identity-point H0")
	}
}

func TestUtxoConstructorRequiresH0OrNullifyingKey(t *testing.T) {
	if _, err := utxo.NewUtxo(types.UtxoParams{Amount: big.NewInt(0), Erc20TokenAddress: constants.ZeroAddress}); err == nil {
		t.Fatal("expected UTXO without H0 or nullifyingKey to fail")
	}
}
