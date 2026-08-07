package keys

import (
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"
)

func SpendingKeyPairJSON(u *cryptokeys.UserKeys) (string, error) {
	pair, err := u.GetSpendingKeyPair()
	if err != nil {
		return "", err
	}
	return codec.JSONString(mobiletypes.SpendingKeyPairJSON{
		PrivSpendingKey:     pair.PrivSpendingKey,
		PubSpendingBJJPoint: []string{codec.EncodeBig(pair.PubSpendingBJJPoint[0]), codec.EncodeBig(pair.PubSpendingBJJPoint[1])},
	})
}

func SignEddsaJSON(u *cryptokeys.UserKeys, message string) (string, error) {
	m, err := codec.DecodeBig(message)
	if err != nil {
		return "", err
	}
	sig, err := u.SignEddsa(m)
	if err != nil {
		return "", err
	}
	return codec.JSONString(mobiletypes.EddsaSignatureJSON{
		R8: []string{codec.EncodeBig(sig.R8[0]), codec.EncodeBig(sig.R8[1])},
		S:  codec.EncodeBig(sig.S),
	})
}

func ShieldedPrivateKeyFromNonce(u *cryptokeys.UserKeys, nonce string) (string, error) {
	n, err := codec.DecodeBig(nonce)
	if err != nil {
		return "", err
	}
	return u.GetShieldedPrivateKeyFromNonce(n)
}

func ClaimableSignatureFromNonce(u *cryptokeys.UserKeys, nonce string) (string, error) {
	n, err := codec.DecodeBig(nonce)
	if err != nil {
		return "", err
	}
	return u.GetClaimableSignatureFromNonce(n)
}

func SignerPrivateKeyFromNonce(u *cryptokeys.UserKeys, walletNonce string) (string, error) {
	n, err := codec.DecodeBig(walletNonce)
	if err != nil {
		return "", err
	}
	return u.GetSignerPrivateKeyFromNonce(n)
}

func SignerSolanaPrivateKeyFromNonce(u *cryptokeys.UserKeys, walletNonce string) (string, error) {
	n, err := codec.DecodeBig(walletNonce)
	if err != nil {
		return "", err
	}
	return u.GetSignerSolanaPrivateKeyFromNonce(n)
}
