package cryptokeys

import (
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/curve25519"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const (
	prefixForSpendingPair   = "1"
	prefixForNullifyingPair = "2"

	shieldedKeyScope        = "hinkal-offchain-shielded-key"
	claimableSignatureScope = "hinkal-claimable-utxo-signature"
)

type UserKeys struct {
	signature     string
	nullifyingKey string
}

func NewUserKeys(signature string) *UserKeys {
	return &UserKeys{signature: signature}
}

func NewUserKeysWithSignatureAndNullifyingKey(signature, nullifyingKey string) *UserKeys {
	return &UserKeys{signature: signature, nullifyingKey: nullifyingKey}
}

func NewUserKeysFromNullifyingKey(nullifyingKey string) *UserKeys {
	return &UserKeys{nullifyingKey: nullifyingKey}
}

func (u *UserKeys) GetSignature() (string, error) {
	if err := u.requireSignature(); err != nil {
		return "", err
	}
	return u.signature, nil
}

func (u *UserKeys) SetSignature(signature string) {
	u.signature = signature
}

func (u *UserKeys) requireSignature() error {
	if u.signature == "" {
		return errors.New("cryptokeys: no signature provided")
	}
	return nil
}

func (u *UserKeys) requireKeyMaterial() error {
	if u.signature == "" && u.nullifyingKey == "" {
		return errors.New("cryptokeys: no signature or private key provided")
	}
	return nil
}

// GetShieldedPrivateKey returns the 0x-prefixed shielded private key derived from the message
// signature used to log in to the application. This private key is used to generate encryption
// keypairs as well as the public key.
func (u *UserKeys) GetShieldedPrivateKey() (string, error) {
	if u.nullifyingKey != "" {
		return u.nullifyingKey, nil
	}
	if err := u.requireKeyMaterial(); err != nil {
		return "", err
	}
	sigBig, err := utils.ParseBigInt(u.signature)
	if err != nil {
		return "", err
	}
	hash, err := crypto.PoseidonBig(sigBig)
	if err != nil {
		return "", err
	}
	hash.Mod(hash, nullifyingPrivateKeyMod)
	u.nullifyingKey = utils.ToBeHex(hash)
	return u.nullifyingKey, nil
}

func (u *UserKeys) shieldedPrivateKeySeed() (string, error) {
	priv, err := u.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	hash := gethcrypto.Keccak256(common.FromHex(priv))
	return "0x" + hex.EncodeToString(hash), nil
}

// GetShieldedPublicKey generates the shielded public key from the shielded private key, returned
// as a 0x-prefixed hexstring.
func (u *UserKeys) GetShieldedPublicKey() (string, error) {
	spk, err := u.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	n, err := utils.ParseBigInt(spk)
	if err != nil {
		return "", err
	}
	h, err := crypto.PoseidonBig(n)
	if err != nil {
		return "", err
	}
	return utils.ToBeHex(h), nil
}

// GetSpendingKeyPair derives the EdDSA-Poseidon BabyJubJub public key for the spending private
// key. This must match the key that SignEddsa (circomlib's signPoseidon) signs with, i.e. the
// scalar is derived from blake512(prv) + clamping, not from privSpendingKey directly.
func (u *UserKeys) GetSpendingKeyPair() (types.SpendingKeyPair, error) {
	if err := u.requireSignature(); err != nil {
		return types.SpendingKeyPair{}, err
	}
	signature, err := u.GetSignature()
	if err != nil {
		return types.SpendingKeyPair{}, err
	}
	prefix, ok := new(big.Int).SetString(prefixForSpendingPair, 10)
	if !ok {
		return types.SpendingKeyPair{}, errors.New("cryptokeys: invalid spending prefix")
	}
	sigBig, err := utils.ParseBigInt(signature)
	if err != nil {
		return types.SpendingKeyPair{}, err
	}
	privSpendingBig, err := crypto.PoseidonBig(prefix, sigBig)
	if err != nil {
		return types.SpendingKeyPair{}, err
	}
	privSpendingKey := utils.ToBeHex(privSpendingBig)
	pub := crypto.EddsaPrv2Pub(common.FromHex(privSpendingKey))
	return types.SpendingKeyPair{
		PrivSpendingKey:     privSpendingKey,
		PubSpendingBJJPoint: types.JubPoint{pub.X, pub.Y},
	}, nil
}

// SignEddsa EdDSA-Poseidon signs a message (a circom field element) using the shielded private key
// as the EdDSA secret. Returns { R8, S } with R8 as a JubPoint and S as a scalar.
func (u *UserKeys) SignEddsa(message *big.Int) (types.EddsaSignature, error) {
	if err := u.requireSignature(); err != nil {
		return types.EddsaSignature{}, err
	}
	pair, err := u.GetSpendingKeyPair()
	if err != nil {
		return types.EddsaSignature{}, err
	}
	msg := new(big.Int).Mod(message, crypto.FieldP)
	r8, s, err := crypto.EddsaSignPoseidon(common.FromHex(pair.PrivSpendingKey), msg)
	if err != nil {
		return types.EddsaSignature{}, err
	}
	return types.EddsaSignature{R8: types.JubPoint{r8.X, r8.Y}, S: s}, nil
}

// GetShieldedPrivateKeyFromNonce is currently used for creating shielded private keys for deposit
// UTXOs, which are encrypted and sent to the enclave for private send. The nonce is used to
// deterministically generate a new shielded private key, returned as a 0x-prefixed hexstring.
func (u *UserKeys) GetShieldedPrivateKeyFromNonce(nonce *big.Int) (string, error) {
	return u.derivedKeyFromNonce(nonce, shieldedKeyScope)
}

func (u *UserKeys) GetClaimableSignatureFromNonce(nonce *big.Int) (string, error) {
	return u.derivedKeyFromNonce(nonce, claimableSignatureScope)
}

func (u *UserKeys) derivedKeyFromNonce(nonce *big.Int, scope string) (string, error) {
	if err := u.requireKeyMaterial(); err != nil {
		return "", err
	}
	seed, err := u.poseidonSeed(nonce)
	if err != nil {
		return "", err
	}
	return keccak256Utf8(scope + ":" + seed), nil
}

// GetSignerPrivateKeyFromNonce generates an access token; this accessKey is what should be sent to
// the server for signing. Returns the accessKey as a 0x-prefixed hexstring.
func (u *UserKeys) GetSignerPrivateKeyFromNonce(walletNonce *big.Int) (string, error) {
	if err := u.requireKeyMaterial(); err != nil {
		return "", err
	}
	seed, err := u.poseidonSeed(walletNonce)
	if err != nil {
		return "", err
	}
	return keccak256Utf8(seed), nil
}

func (u *UserKeys) GetSignerSolanaPrivateKeyFromNonce(walletNonce *big.Int) (string, error) {
	privateKey, err := u.GetSignerPrivateKeyFromNonce(walletNonce)
	if err != nil {
		return "", err
	}
	seed := common.FromHex(privateKey)
	if len(seed) != ed25519.SeedSize {
		return "", errors.New("cryptokeys: signer private key is not a 32-byte seed")
	}
	return base58.Encode(ed25519.NewKeyFromSeed(seed)), nil
}

func (u *UserKeys) poseidonSeed(nonce *big.Int) (string, error) {
	priv, err := u.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	pub, err := u.GetShieldedPublicKey()
	if err != nil {
		return "", err
	}
	privBig, err := utils.ParseBigInt(priv)
	if err != nil {
		return "", err
	}
	pubBig, err := utils.ParseBigInt(pub)
	if err != nil {
		return "", err
	}
	seed, err := crypto.PoseidonBig(nonce, privBig, pubBig)
	if err != nil {
		return "", err
	}
	return utils.ToBeHex(seed), nil
}

// GetDerivedEthereumAddress deterministically derives an EVM address from the shielded private key.
func (u *UserKeys) GetDerivedEthereumAddress() (string, error) {
	priv, err := u.shieldedPrivateKeySeed()
	if err != nil {
		return "", err
	}
	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(priv, "0x"))
	if err != nil {
		return "", err
	}
	return gethcrypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}

// GetDerivedSolanaPublicKey deterministically derives a Solana public key from the shielded private key.
func (u *UserKeys) GetDerivedSolanaPublicKey() (string, error) {
	keypair, err := u.derivedSolanaKeypair()
	if err != nil {
		return "", err
	}
	return base58.Encode(keypair[ed25519.SeedSize:]), nil
}

// GetNearIntentsAccountID deterministically derives the user's NEAR Intents account id from the
// shielded private key. The account id is the NEAR implicit account (lowercase hex of the ed25519
// public key) of the same keypair returned by GetDerivedSolanaPublicKey. Used as a private
// cross-chain refund destination so refunds never expose the user's public main wallet.
func (u *UserKeys) GetNearIntentsAccountID() (string, error) {
	keypair, err := u.derivedSolanaKeypair()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(keypair[ed25519.SeedSize:]), nil
}

// GetNearIntentsKeyPairString deterministically derives the NEAR Intents signing key from the
// shielded private key, formatted as a near-api-js KeyPair string ("ed25519:<base58 secret>").
// Pairs with GetNearIntentsAccountID and is used client-side to authorize claiming cross-chain
// bridge refunds back out of the user's Intents account.
func (u *UserKeys) GetNearIntentsKeyPairString() (string, error) {
	keypair, err := u.derivedSolanaKeypair()
	if err != nil {
		return "", err
	}
	return "ed25519:" + base58.Encode(keypair), nil
}

func (u *UserKeys) derivedSolanaKeypair() (ed25519.PrivateKey, error) {
	if err := u.requireKeyMaterial(); err != nil {
		return nil, err
	}
	priv, err := u.shieldedPrivateKeySeed()
	if err != nil {
		return nil, err
	}
	seed := common.FromHex(priv)
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New("cryptokeys: shielded private key is not a 32-byte seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func (u *UserKeys) VerifyMessage(signingMessage string) (string, error) {
	if err := u.requireSignature(); err != nil {
		return "", err
	}
	sig, err := recoverableSignature(u.signature)
	if err != nil {
		return "", err
	}
	pub, err := gethcrypto.SigToPub(accounts.TextHash([]byte(signingMessage)), sig)
	if err != nil {
		return "", err
	}
	return gethcrypto.PubkeyToAddress(*pub).Hex(), nil
}

func (u *UserKeys) VerifySolanaMessage(signingMessage, signerPublicKey string) (bool, error) {
	if err := u.requireSignature(); err != nil {
		return false, err
	}
	pubBytes, err := base58.Decode(signerPublicKey)
	if err != nil {
		return false, err
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return false, errors.New("cryptokeys: invalid solana public key length")
	}
	return ed25519.Verify(pubBytes, []byte(signingMessage), common.FromHex(u.signature)), nil
}

func (u *UserKeys) VerifyTronMessage(signingMessage, signerAddress string) (bool, error) {
	if err := u.requireSignature(); err != nil {
		return false, err
	}
	sig, err := recoverableSignature(u.signature)
	if err != nil {
		return false, err
	}
	msg := []byte(signingMessage)
	prefix := "\x19TRON Signed Message:\n" + strconv.Itoa(len(msg))
	hash := gethcrypto.Keccak256(append([]byte(prefix), msg...))
	pub, err := gethcrypto.SigToPub(hash, sig)
	if err != nil {
		return false, err
	}
	recoveredTron, err := utils.EVMHexToTronBase58Address(gethcrypto.PubkeyToAddress(*pub).Hex())
	if err != nil {
		return false, err
	}
	expectedTron, err := utils.ToTronBase58IfHex(signerAddress)
	if err != nil {
		return false, err
	}
	return recoveredTron == expectedTron, nil
}

// GetAccessKey generates the access token; this accessKey is what should be sent to the server for
// signing. Returns the accessKey as a 0x-prefixed hexstring.
func (u *UserKeys) GetAccessKey() (string, error) {
	if err := u.requireKeyMaterial(); err != nil {
		return "", err
	}
	priv, err := u.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	pub, err := u.GetShieldedPublicKey()
	if err != nil {
		return "", err
	}
	return poseidonHashHex(priv, pub)
}

// GetBackendToken generates the backend token, which is used for access control on the backend.
// Returns the token as a 0x-prefixed hexstring.
func (u *UserKeys) GetBackendToken() (string, error) {
	if err := u.requireKeyMaterial(); err != nil {
		return "", err
	}
	accessKey, err := u.GetAccessKey()
	if err != nil {
		return "", err
	}
	pub, err := u.GetShieldedPublicKey()
	if err != nil {
		return "", err
	}
	return poseidonHashHex(accessKey, pub)
}

func GetRandomizedStealthPair(s *big.Int, privateKey string) (types.JubPoint, types.JubPoint, error) {
	pair, err := stealthPairCache.GetOrSet(stealthCacheKey(s, privateKey), func() (cachedStealthPair, error) {
		privateKeyBig, err := utils.ParseBigInt(privateKey)
		if err != nil {
			return cachedStealthPair{}, err
		}
		h0 := babyjub.NewPoint().Mul(s, babyjub.B8)
		// H1 = H0 * privKey, matching the circuit's EscalarMulAny, not Base8 * ((s*privKey) mod P).
		h1 := babyjub.NewPoint().Mul(privateKeyBig, h0)
		return cachedStealthPair{types.JubPoint{h0.X, h0.Y}, types.JubPoint{h1.X, h1.Y}}, nil
	})
	if err != nil {
		return types.JubPoint{}, types.JubPoint{}, err
	}
	return copyJubPoint(pair.h0), copyJubPoint(pair.h1), nil
}

func GetH1FromH0(h0 types.JubPoint, privateKey string) (types.JubPoint, error) {
	privateKeyBig, err := utils.ParseBigInt(privateKey)
	if err != nil {
		return types.JubPoint{}, err
	}
	h0Point := &babyjub.Point{X: new(big.Int).Set(h0[0]), Y: new(big.Int).Set(h0[1])}
	h1 := babyjub.NewPoint().Mul(privateKeyBig, h0Point)
	return types.JubPoint{h1.X, h1.Y}, nil
}

func VerifyStealthPair(h0, h1 types.JubPoint, privateKey string, onlyYCheck bool) (bool, error) {
	candidate, err := GetH1FromH0(h0, privateKey)
	if err != nil {
		return false, err
	}
	if onlyYCheck {
		return candidate[1].Cmp(h1[1]) == 0, nil
	}
	return candidate[0].Cmp(h1[0]) == 0 && candidate[1].Cmp(h1[1]) == 0, nil
}

func GetStealthAddress(h0 types.JubPoint, nullifyingPrivateKey string, spendingPublicKey []*big.Int) (string, error) {
	if len(spendingPublicKey) != 2 {
		return "", errors.New("cryptokeys: spending public key must be an array of 2 elements")
	}
	h1, err := GetH1FromH0(h0, nullifyingPrivateKey)
	if err != nil {
		return "", err
	}
	nullifyingBig, err := utils.ParseBigInt(nullifyingPrivateKey)
	if err != nil {
		return "", err
	}
	signs := new(big.Int).Add(new(big.Int).Mul(big.NewInt(2), getCircomSign(h0[0])), getCircomSign(h1[0]))
	h, err := crypto.PoseidonBig(signs, h0[1], h1[1], spendingPublicKey[0], spendingPublicKey[1], nullifyingBig)
	if err != nil {
		return "", err
	}
	return utils.ToBeHex(h), nil
}

func FindCorrectRandomization(initialRandomization *big.Int, privateKey string) (*big.Int, error) {
	privateKeyBig, err := utils.ParseBigInt(privateKey)
	if err != nil {
		return nil, err
	}
	ten := big.NewInt(10)
	i := big.NewInt(0)
	var s, sPk *big.Int
	for {
		pow := new(big.Int).Exp(ten, i, nil)
		s = new(big.Int).Mul(initialRandomization, pow)
		s.Mod(s, circomP)
		sPk = new(big.Int).Mul(s, privateKeyBig)
		sPk.Mod(sPk, circomP)
		i.Add(i, big.NewInt(1))
		// within 250 bits: BabyPbk() inputs are within 250 bits
		if sPk.Cmp(nullifyingPrivateKeyMod) < 0 && s.Cmp(nullifyingPrivateKeyMod) < 0 {
			break
		}
	}
	return s, nil
}

func GetH0FromRandomization(randomization *big.Int) types.JubPoint {
	h0 := babyjub.NewPoint().Mul(randomization, babyjub.B8)
	return types.JubPoint{h0.X, h0.Y}
}

func FindH0(randomization *big.Int, privateKey string) (*types.JubPoint, error) {
	if privateKey == "" {
		return nil, nil
	}
	h0, _, err := GetRandomizedStealthPair(randomization, privateKey)
	if err != nil {
		return nil, err
	}
	return &h0, nil
}

// GetEncryptionKeyPair generates a private and public keypair. The seed must be in DataHexString
// format and must correspond to 32 bytes.
func GetEncryptionKeyPair(privateKey string) (types.EncryptionKeyPairHex, error) {
	sk, pk, err := EncryptionKeyPair(privateKey)
	if err != nil {
		return types.EncryptionKeyPairHex{}, err
	}
	return types.EncryptionKeyPairHex{
		PrivateKey: "0x" + hex.EncodeToString(sk[:]),
		PublicKey:  "0x" + hex.EncodeToString(pk[:]),
	}, nil
}

// EncryptionKeyPair derives a curve25519 keypair from seedHex, matching libsodium's
// crypto_box_seed_keypair(sha512(seed)[:32]). seedHex is re-expanded via keccak256 first since
// callers pass the shielded/nullifying private key, which is now constrained to < 2^250 (not a
// full 32-byte seed). Mirrors getEncryptionKeyPair in keys.ts.
func EncryptionKeyPair(seedHex string) (sk, pk [32]byte, err error) {
	seed := gethcrypto.Keccak256(common.FromHex(seedHex))
	hashed := sha512.Sum512(seed)
	copy(sk[:], hashed[:32])
	pkBytes, e := curve25519.X25519(sk[:], curve25519.Basepoint)
	if e != nil {
		return sk, pk, e
	}
	copy(pk[:], pkBytes)
	return sk, pk, nil
}
