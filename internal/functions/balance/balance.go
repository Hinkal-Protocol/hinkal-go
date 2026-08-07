package balance

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type Result struct {
	Balances map[string]*big.Int
	Utxos    []*utxo.Utxo
}

func Compute(
	encryptedOutputs []*types.EncryptedOutputWithSign,
	nullifiers map[string]struct{},
	uk *cryptokeys.UserKeys,
	chainID int,
) (*Result, error) {
	res := &Result{Balances: map[string]*big.Int{}}

	for _, out := range encryptedOutputs {
		u, err := decodeOutput(out, uk, chainID)
		if err != nil {
			continue
		}
		u.IsBlocked = out.IsBlocked

		nul, err := u.GetNullifier()
		if err != nil {
			continue
		}
		if _, spent := nullifiers[nul]; spent {
			continue
		}
		if u.IsBlocked {
			continue
		}

		token, err := balanceTokenKey(u, chainID)
		if err != nil {
			continue
		}
		if res.Balances[token] == nil {
			res.Balances[token] = new(big.Int)
		}
		res.Balances[token].Add(res.Balances[token], u.Amount)
		res.Utxos = append(res.Utxos, u)
	}

	return res, nil
}

func decodeOutput(out *types.EncryptedOutputWithSign, uk *cryptokeys.UserKeys, chainID int) (*utxo.Utxo, error) {
	if out.IsPositive {
		return DecryptUtxoHex(out.Value, uk)
	}
	return DecodeUtxo(out.Value, uk, chainID)
}

func balanceTokenKey(u *utxo.Utxo, chainID int) (string, error) {
	if constants.IsSolanaLike(chainID) || constants.IsTronLike(chainID) {
		return u.GetTokenAddress(chainID)
	}
	return common.HexToAddress(u.Erc20TokenAddress).Hex(), nil
}
