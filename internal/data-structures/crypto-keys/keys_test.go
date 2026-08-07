package cryptokeys_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/mr-tron/base58"

	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func sampleSignature() string {
	return "0x" + strings.Repeat("ab", 65)
}

func TestShieldedKeyDeterministicAndCached(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())
	priv1, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	priv2, _ := uk.GetShieldedPrivateKey()
	if priv1 != priv2 || !strings.HasPrefix(priv1, "0x") || len(priv1) != 66 {
		t.Fatalf("unexpected shielded private key %q", priv1)
	}

	fromNullifying := cryptokeys.NewUserKeysFromNullifyingKey(priv1)
	got, _ := fromNullifying.GetShieldedPrivateKey()
	if got != priv1 {
		t.Fatalf("nullifying-key constructor mismatch: %q != %q", got, priv1)
	}
}

func TestNoKeyMaterialErrors(t *testing.T) {
	uk := cryptokeys.NewUserKeys("")
	if _, err := uk.GetShieldedPrivateKey(); err == nil {
		t.Fatal("expected error with no key material")
	}
	if _, err := uk.GetSignature(); err == nil {
		t.Fatal("expected error with no signature")
	}
}

func TestSignEddsaVerifiesAgainstSpendingKey(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())
	pair, err := uk.GetSpendingKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	pubPoint := &babyjub.Point{X: pair.PubSpendingBJJPoint[0], Y: pair.PubSpendingBJJPoint[1]}
	if !pubPoint.InSubGroup() {
		t.Fatal("spending public key not in subgroup")
	}

	msg := big.NewInt(987654321)
	sig, err := uk.SignEddsa(msg)
	if err != nil {
		t.Fatal(err)
	}
	pk := babyjub.PublicKey(*pubPoint)
	ok := pk.VerifyPoseidon(msg, &babyjub.Signature{
		R8: &babyjub.Point{X: sig.R8[0], Y: sig.R8[1]},
		S:  sig.S,
	})
	if !ok {
		t.Fatal("eddsa signature failed to verify")
	}
}

func TestNonceDerivationsScopedAndDeterministic(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())
	nonce := big.NewInt(5)

	shielded, _ := uk.GetShieldedPrivateKeyFromNonce(nonce)
	shieldedAgain, _ := uk.GetShieldedPrivateKeyFromNonce(nonce)
	claimable, _ := uk.GetClaimableSignatureFromNonce(nonce)
	signer, _ := uk.GetSignerPrivateKeyFromNonce(nonce)

	if shielded != shieldedAgain {
		t.Fatal("nonce derivation not deterministic")
	}
	for _, pair := range [][2]string{{shielded, claimable}, {shielded, signer}, {claimable, signer}} {
		if pair[0] == pair[1] {
			t.Fatalf("expected distinct derivations, both = %q", pair[0])
		}
	}
	other, _ := uk.GetShieldedPrivateKeyFromNonce(big.NewInt(6))
	if other == shielded {
		t.Fatal("different nonces produced same key")
	}
}

func TestDerivedAddresses(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())

	eth, err := uk.GetDerivedEthereumAddress()
	if err != nil {
		t.Fatal(err)
	}
	if eth != common.HexToAddress(eth).Hex() {
		t.Fatalf("eth address not EIP-55 checksummed: %q", eth)
	}

	sol, err := uk.GetDerivedSolanaPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	solBytes, err := base58.Decode(sol)
	if err != nil || len(solBytes) != ed25519.PublicKeySize {
		t.Fatalf("invalid solana public key %q", sol)
	}

	accountID, _ := uk.GetNearIntentsAccountID()
	if accountID != hex.EncodeToString(solBytes) {
		t.Fatalf("near account id %q does not match solana pubkey", accountID)
	}

	keyPair, _ := uk.GetNearIntentsKeyPairString()
	if !strings.HasPrefix(keyPair, "ed25519:") {
		t.Fatalf("near keypair string missing prefix: %q", keyPair)
	}
	secret, err := base58.Decode(strings.TrimPrefix(keyPair, "ed25519:"))
	if err != nil || len(secret) != ed25519.PrivateKeySize {
		t.Fatalf("invalid near secret key length %d", len(secret))
	}
	if !strings.HasSuffix(hex.EncodeToString(secret), accountID) {
		t.Fatal("near secret key public half does not match account id")
	}
}

func TestVerifyMessage(t *testing.T) {
	key, _ := gethcrypto.GenerateKey()
	msg := "login to hinkal"
	sig, err := gethcrypto.Sign(accounts.TextHash([]byte(msg)), key)
	if err != nil {
		t.Fatal(err)
	}
	uk := cryptokeys.NewUserKeys("0x" + hex.EncodeToString(sig))

	got, err := uk.VerifyMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	want := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	if got != want {
		t.Fatalf("recovered %q want %q", got, want)
	}
	if wrong, _ := uk.VerifyMessage("tampered"); wrong == want {
		t.Fatal("tampered message recovered same signer")
	}
}

func TestVerifySolanaMessage(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	msg := "solana login"
	sig := ed25519.Sign(priv, []byte(msg))
	uk := cryptokeys.NewUserKeys("0x" + hex.EncodeToString(sig))

	ok, err := uk.VerifySolanaMessage(msg, base58.Encode(pub))
	if err != nil || !ok {
		t.Fatalf("valid solana signature rejected (err=%v)", err)
	}
	if bad, _ := uk.VerifySolanaMessage("tampered", base58.Encode(pub)); bad {
		t.Fatal("tampered solana message verified")
	}
}

func TestVerifyTronMessage(t *testing.T) {
	key, _ := gethcrypto.GenerateKey()
	evm := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	tronAddr, err := utils.EVMHexToTronBase58Address(evm)
	if err != nil {
		t.Fatal(err)
	}
	msg := "tron login"
	prefix := "\x19TRON Signed Message:\n" + strconv.Itoa(len(msg))
	hash := gethcrypto.Keccak256(append([]byte(prefix), []byte(msg)...))
	sig, _ := gethcrypto.Sign(hash, key)
	uk := cryptokeys.NewUserKeys("0x" + hex.EncodeToString(sig))

	ok, err := uk.VerifyTronMessage(msg, tronAddr)
	if err != nil || !ok {
		t.Fatalf("valid tron signature rejected (err=%v)", err)
	}
	if bad, _ := uk.VerifyTronMessage("tampered", tronAddr); bad {
		t.Fatal("tampered tron message verified")
	}
}

func TestStealthNewStyleRoundTrip(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())
	priv, _ := uk.GetShieldedPrivateKey()
	s := big.NewInt(424242)

	H0, H1, err := cryptokeys.GetRandomizedStealthPair(s, priv)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := cryptokeys.VerifyStealthPair(H0, H1, priv, true)
	if err != nil || !ok {
		t.Fatalf("new-style stealth pair rejected (err=%v)", err)
	}
	okFull, _ := cryptokeys.VerifyStealthPair(H0, H1, priv, false)
	if !okFull {
		t.Fatal("full stealth pair check failed")
	}

	derived, err := cryptokeys.GetH1FromH0(H0, priv)
	if err != nil {
		t.Fatal(err)
	}
	if derived[0].Cmp(H1[0]) != 0 || derived[1].Cmp(H1[1]) != 0 {
		t.Fatal("GetH1FromH0 disagrees with GetRandomizedStealthPair")
	}

	pair, _ := uk.GetSpendingKeyPair()
	spend := []*big.Int{pair.PubSpendingBJJPoint[0], pair.PubSpendingBJJPoint[1]}
	if _, err := cryptokeys.GetStealthAddress(H0, priv, spend); err != nil {
		t.Fatal(err)
	}
	if _, err := cryptokeys.GetStealthAddress(H0, priv, spend[:1]); err == nil {
		t.Fatal("expected error for malformed spending key")
	}
}

func TestRandomizationPointOnCurve(t *testing.T) {
	H0 := cryptokeys.GetH0FromRandomization(big.NewInt(13))
	point := &babyjub.Point{X: H0[0], Y: H0[1]}
	if !point.InSubGroup() {
		t.Fatal("H0 not in subgroup")
	}

	found, err := cryptokeys.FindH0(big.NewInt(0), "")
	if err != nil || found != nil {
		t.Fatal("FindH0 should return nil with empty private key")
	}
}

func TestEncryptionKeyPairDeterministic(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())
	priv, _ := uk.GetShieldedPrivateKey()

	kp, err := cryptokeys.GetEncryptionKeyPair(priv)
	if err != nil {
		t.Fatal(err)
	}
	kp2, _ := cryptokeys.GetEncryptionKeyPair(priv)
	if kp != kp2 {
		t.Fatal("encryption keypair not deterministic")
	}
	sk, pk, _ := cryptokeys.EncryptionKeyPair(priv)
	if kp.PrivateKey != "0x"+hex.EncodeToString(sk[:]) || kp.PublicKey != "0x"+hex.EncodeToString(pk[:]) {
		t.Fatal("hex keypair does not match raw EncryptionKeyPair")
	}
}

func TestAccessAndBackendTokens(t *testing.T) {
	uk := cryptokeys.NewUserKeys(sampleSignature())
	access, err := uk.GetAccessKey()
	if err != nil {
		t.Fatal(err)
	}
	backend, err := uk.GetBackendToken()
	if err != nil {
		t.Fatal(err)
	}
	if access == backend || !strings.HasPrefix(access, "0x") || !strings.HasPrefix(backend, "0x") {
		t.Fatalf("unexpected tokens access=%q backend=%q", access, backend)
	}
}
