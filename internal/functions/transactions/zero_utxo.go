package transactions

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func buildZeroUtxo(hinkal ihinkal.HinkalInternal, tokenAddress, timeStamp string) (*utxo.Utxo, error) {
	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	params := types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: tokenAddress,
		NullifyingKey:     shieldedPrivateKey,
		TimeStamp:         timeStamp,
	}
	spendingKeyPair, err := hinkal.GetUserKeys().GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	params.SpendingPublicKey = []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	return utxo.NewUtxo(params)
}

func buildSolanaZeroUtxo(hinkal ihinkal.HinkalInternal, mintAddress, timeStamp string) (*utxo.Utxo, error) {
	nativeMint, err := solanautils.FormatMintAddress(constants.SolanaNativeAddress)
	if err != nil {
		return nil, err
	}
	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	params := types.UtxoParams{
		Amount:            big.NewInt(0),
		MintAddress:       mintAddress,
		Erc20TokenAddress: nativeMint.CompressedAddress,
		NullifyingKey:     shieldedPrivateKey,
		TimeStamp:         timeStamp,
	}
	spendingKeyPair, err := hinkal.GetUserKeys().GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	params.SpendingPublicKey = []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	return utxo.NewUtxo(params)
}
