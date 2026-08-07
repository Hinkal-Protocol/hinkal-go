package pretransaction

import (
	"context"
	"errors"
	"math/big"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
)

func CalculateSolanaNullifierCount(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) int {
	if !constants.IsSolanaLike(chainID) || len(mintAddresses) == 0 || len(amountChanges) == 0 {
		return 0
	}
	count, err := solanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges)
	if err != nil {
		var utxoLimitErr *errorhandling.ErrorWithAmount
		if errors.As(err, &utxoLimitErr) {
			return 6
		}
		return 0
	}
	return count
}

func solanaNullifierCount(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) (int, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, false, false)
	if err != nil {
		return 0, err
	}

	var nonZeroUtxos []*utxo.Utxo
	for _, group := range inputUtxosArray {
		for _, u := range group {
			if u.Amount != nil && u.Amount.Sign() != 0 {
				nonZeroUtxos = append(nonZeroUtxos, u)
			}
		}
	}
	if len(nonZeroUtxos) == 0 {
		return 0, nil
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return 0, err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return 0, err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return 0, err
	}

	pdasToCheck := make([]solana.PublicKey, len(nonZeroUtxos))
	for i, u := range nonZeroUtxos {
		nullifierStr, err := u.GetNullifier()
		if err != nil {
			return 0, err
		}
		nullifier, err := utils.ParseBigInt(nullifierStr)
		if err != nil {
			return 0, err
		}
		pda, err := web3.GetNullifierAccount(nullifier, originalDeployer, programID)
		if err != nil {
			return 0, err
		}
		pdasToCheck[i] = pda
	}

	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return 0, err
	}
	res, err := connection.GetMultipleAccounts(ctx, pdasToCheck...)
	if err != nil {
		return 0, err
	}

	missing := 0
	for _, account := range res.Value {
		if account == nil {
			missing++
		}
	}
	return missing, nil
}
