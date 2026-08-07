package mobile

import (
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	keys "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/keys"
)

type UserKeys struct {
	u *cryptokeys.UserKeys
}

func NewUserKeys(signature string) *UserKeys {
	return &UserKeys{u: cryptokeys.NewUserKeys(signature)}
}

func NewUserKeysWithSignatureAndNullifyingKey(signature, nullifyingKey string) *UserKeys {
	return &UserKeys{u: cryptokeys.NewUserKeysWithSignatureAndNullifyingKey(signature, nullifyingKey)}
}

func UserKeysFromNullifyingKey(nullifyingKey string) *UserKeys {
	return &UserKeys{u: cryptokeys.NewUserKeysFromNullifyingKey(nullifyingKey)}
}

func (k *UserKeys) GetSignature() (string, error) {
	return k.u.GetSignature()
}

func (k *UserKeys) SetSignature(signature string) {
	k.u.SetSignature(signature)
}

func (k *UserKeys) GetShieldedPrivateKey() (string, error) {
	return k.u.GetShieldedPrivateKey()
}

func (k *UserKeys) GetShieldedPublicKey() (string, error) {
	return k.u.GetShieldedPublicKey()
}

func (k *UserKeys) GetSpendingKeyPairJSON() (string, error) {
	return keys.SpendingKeyPairJSON(k.u)
}

func (k *UserKeys) GetShieldedPrivateKeyFromNonce(nonce string) (string, error) {
	return keys.ShieldedPrivateKeyFromNonce(k.u, nonce)
}

func (k *UserKeys) GetClaimableSignatureFromNonce(nonce string) (string, error) {
	return keys.ClaimableSignatureFromNonce(k.u, nonce)
}

func (k *UserKeys) GetSignerPrivateKeyFromNonce(walletNonce string) (string, error) {
	return keys.SignerPrivateKeyFromNonce(k.u, walletNonce)
}

func (k *UserKeys) GetSignerSolanaPrivateKeyFromNonce(walletNonce string) (string, error) {
	return keys.SignerSolanaPrivateKeyFromNonce(k.u, walletNonce)
}

func (k *UserKeys) GetDerivedEthereumAddress() (string, error) {
	return k.u.GetDerivedEthereumAddress()
}

func (k *UserKeys) GetDerivedSolanaPublicKey() (string, error) {
	return k.u.GetDerivedSolanaPublicKey()
}

func (k *UserKeys) GetNearIntentsAccountID() (string, error) {
	return k.u.GetNearIntentsAccountID()
}

func (k *UserKeys) GetNearIntentsKeyPairString() (string, error) {
	return k.u.GetNearIntentsKeyPairString()
}

func (k *UserKeys) VerifyMessage(signingMessage string) (string, error) {
	return k.u.VerifyMessage(signingMessage)
}

func (k *UserKeys) VerifySolanaMessage(signingMessage, signerPublicKey string) (bool, error) {
	return k.u.VerifySolanaMessage(signingMessage, signerPublicKey)
}

func (k *UserKeys) VerifyTronMessage(signingMessage, signerAddress string) (bool, error) {
	return k.u.VerifyTronMessage(signingMessage, signerAddress)
}

func (k *UserKeys) GetAccessKey() (string, error) {
	return k.u.GetAccessKey()
}

func (k *UserKeys) GetBackendToken() (string, error) {
	return k.u.GetBackendToken()
}

func (k *UserKeys) SignEddsaJSON(message string) (string, error) {
	return keys.SignEddsaJSON(k.u, message)
}
