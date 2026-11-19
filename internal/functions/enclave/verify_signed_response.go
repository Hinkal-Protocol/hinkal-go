package enclave

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const (
	attestationIssuer        = "https://confidentialcomputing.googleapis.com"
	confidentialSpaceJWKSURI = "https://www.googleapis.com/service_accounts/v1/metadata/jwk/signer@confidentialspace-sign.iam.gserviceaccount.com"
	clockSkewSeconds         = 300
	googleKeysTTL            = time.Hour
)

var errUnexpectedEnclaveSigner = errors.New("enclave: response signature verification failed: unexpected signer")

type googleJWK struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type attestationClaims struct {
	Iss                   string          `json:"iss"`
	Aud                   json.RawMessage `json:"aud"`
	Exp                   int64           `json:"exp"`
	HwModel               string          `json:"hwmodel"`
	SwName                string          `json:"swname"`
	DbgStat               string          `json:"dbgstat"`
	GoogleServiceAccounts []string        `json:"google_service_accounts"`
	Submods               struct {
		ConfidentialSpace struct {
			SupportAttributes []string `json:"support_attributes"`
		} `json:"confidential_space"`
	} `json:"submods"`
}

var (
	attestationMu       sync.Mutex
	googleKeys          []googleJWK
	googleKeysFetchedAt time.Time
	lastAttested        struct {
		token, signer string
		exp           int64
	}
)

func attestationRejected(format string, args ...any) error {
	return fmt.Errorf("enclave: attestation rejected: "+format, args...)
}

func verifySignedEnclaveResponse[T any](ctx context.Context, signed types.SignedEnclaveResponse) (T, error) {
	var dest T

	if signed.AttestedSignature == "" || signed.Attestation == "" {
		return dest, attestationRejected("the response carries no attestation")
	}
	signer, err := verifyAttestation(ctx, signed.Attestation)
	if err != nil {
		return dest, err
	}
	if !strings.EqualFold(recoverSigner(signed.Data, signed.AttestedSignature), signer) {
		return dest, errUnexpectedEnclaveSigner
	}

	if err := json.Unmarshal([]byte(signed.Data), &dest); err != nil {
		return dest, fmt.Errorf("enclave: parse response data: %w", err)
	}
	return dest, nil
}

// verifyAttestation checks a Confidential Space token and returns the signing address it attests.
// Every henclave instance refreshes one token per signing key, so the last verified token almost
// always matches and costs nothing after the first response.
func verifyAttestation(ctx context.Context, token string) (string, error) {
	attestationMu.Lock()
	defer attestationMu.Unlock()

	now := time.Now().Unix()
	if lastAttested.token == token && lastAttested.exp+clockSkewSeconds > now {
		return lastAttested.signer, nil
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", attestationRejected("it is not a JWT")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return "", err
	}
	if header.Alg != "RS256" {
		return "", attestationRejected("it is signed with %s", header.Alg)
	}
	key, err := googleKey(ctx, header.Kid)
	if err != nil {
		return "", err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", attestationRejected("its signature is not base64url: %v", err)
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return "", attestationRejected("its Google signature is invalid")
	}

	var claims attestationClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return "", err
	}
	signer, err := checkAttestationClaims(claims, now)
	if err != nil {
		return "", err
	}
	lastAttested.token, lastAttested.signer, lastAttested.exp = token, signer, claims.Exp
	return signer, nil
}

// checkAttestationClaims ties the token to henclave: a non-debug SEV VM on a supported image, running
// as henclave's attestation service account, which only the enclave's operator account may mint
// tokens for.
func checkAttestationClaims(claims attestationClaims, now int64) (string, error) {
	serviceAccount := constants.GetEnclaveAttestationServiceAccount()
	switch {
	case claims.Iss != attestationIssuer:
		return "", attestationRejected("it is issued by %s", claims.Iss)
	case claims.Exp+clockSkewSeconds <= now:
		return "", attestationRejected("it has expired")
	case claims.SwName != "CONFIDENTIAL_SPACE" || claims.HwModel != "GCP_AMD_SEV":
		return "", attestationRejected("it attests %s on %s", claims.SwName, claims.HwModel)
	case claims.DbgStat != "disabled-since-boot":
		return "", attestationRejected("the VM booted in debug mode")
	case !slices.Contains(claims.Submods.ConfidentialSpace.SupportAttributes, "STABLE"):
		return "", attestationRejected("the image is not a hardened Confidential Space release")
	case !slices.Contains(claims.GoogleServiceAccounts, serviceAccount):
		return "", attestationRejected("the VM runs as %v, not %s", claims.GoogleServiceAccounts, serviceAccount)
	}

	var audiences []string
	var single string
	if err := json.Unmarshal(claims.Aud, &single); err == nil {
		audiences = []string{single}
	} else if err := json.Unmarshal(claims.Aud, &audiences); err != nil {
		return "", attestationRejected("its audience is malformed")
	}
	if len(audiences) != 1 || audiences[0] == "" {
		return "", attestationRejected("it does not name one signing key")
	}
	return audiences[0], nil
}

// googleKey looks kid up in Google's published keys, refetching on a miss or once they are older
// than googleKeysTTL, since Google rotates and withdraws them. Callers hold attestationMu.
func googleKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	jwk := findJWK(kid)
	if jwk == nil || time.Since(googleKeysFetchedAt) > googleKeysTTL {
		var resp struct {
			Keys []googleJWK `json:"keys"`
		}
		if err := api.Get(ctx, confidentialSpaceJWKSURI, &resp); err != nil {
			return nil, fmt.Errorf("enclave: fetch Google attestation keys: %w", err)
		}
		googleKeys, googleKeysFetchedAt = resp.Keys, time.Now()
		jwk = findJWK(kid)
	}
	if jwk == nil {
		return nil, attestationRejected("it is signed by unknown Google key %s", kid)
	}
	n, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("enclave: decode Google key modulus: %w", err)
	}
	e, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("enclave: decode Google key exponent: %w", err)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
}

func findJWK(kid string) *googleJWK {
	for i := range googleKeys {
		if googleKeys[i].Kid == kid {
			return &googleKeys[i]
		}
	}
	return nil
}

func decodeJWTPart(part string, dest any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return attestationRejected("it is not base64url: %v", err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return attestationRejected("it is not JSON: %v", err)
	}
	return nil
}

// recoverSigner returns "" for a malformed signature, so it reads as an unexpected signer.
func recoverSigner(data, signature string) string {
	sig, err := recoverableSignature(signature)
	if err != nil {
		return ""
	}
	pub, err := gethcrypto.SigToPub(gethcrypto.Keccak256([]byte(data)), sig)
	if err != nil {
		return ""
	}
	return gethcrypto.PubkeyToAddress(*pub).Hex()
}

func recoverableSignature(signature string) ([]byte, error) {
	sig := common.FromHex(signature)
	if len(sig) != 65 {
		return nil, errors.New("enclave: signature must be 65 bytes")
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	return sig, nil
}
