package pretransaction

import (
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
)

func EnsureAmountChanges(inputUtxos, outputUtxos [][]*utxo.Utxo, amountChanges []*big.Int) error {
	diffAmountChanges := snarkjs.CalcAmountChanges(inputUtxos, outputUtxos, false)
	if len(diffAmountChanges) != len(amountChanges) {
		return fmt.Errorf("amount changes are not equal")
	}
	for i, amount := range amountChanges {
		normalized := amount
		if normalized.Sign() < 0 {
			normalized = new(big.Int).Add(crypto.FieldP, normalized)
		}
		if diffAmountChanges[i].Cmp(normalized) != 0 {
			return fmt.Errorf("amount changes are not equal")
		}
	}
	return nil
}
