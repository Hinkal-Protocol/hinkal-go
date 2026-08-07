package enclave

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

var errUnexpectedEnclaveSigner = errors.New("enclave: response signature verification failed: unexpected signer")

func verifySignedEnclaveResponse[T any](data, signature string) (T, error) {
	var dest T

	sig, err := recoverableSignature(signature)
	if err != nil {
		return dest, err
	}
	digest := gethcrypto.Keccak256([]byte(data))
	pub, err := gethcrypto.SigToPub(digest, sig)
	if err != nil {
		return dest, fmt.Errorf("enclave: recover response signer: %w", err)
	}
	recovered := gethcrypto.PubkeyToAddress(*pub).Hex()
	if !strings.EqualFold(recovered, constants.EnclaveSignerAddress) {
		return dest, errUnexpectedEnclaveSigner
	}

	if err := json.Unmarshal([]byte(data), &dest); err != nil {
		return dest, fmt.Errorf("enclave: parse response data: %w", err)
	}
	return dest, nil
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
