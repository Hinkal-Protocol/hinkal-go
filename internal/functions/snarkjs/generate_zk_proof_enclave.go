package snarkjs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type ZkProofResult struct {
	ZkCallData    types.NewZkCallDataType
	PublicSignals []string
}

// toJSONSafe recursively converts *big.Int values to decimal strings so JSON serialization matches
// TS safeJsonStringify, which stringifies every bigint.
func toJSONSafe(v any) any {
	if v == nil {
		return nil
	}
	if b, ok := v.(*big.Int); ok {
		return b.String()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = toJSONSafe(rv.Index(i).Interface())
		}
		return out
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		for _, k := range rv.MapKeys() {
			out[fmt.Sprint(k.Interface())] = toJSONSafe(rv.MapIndex(k).Interface())
		}
		return out
	default:
		return v
	}
}

func retrieveVerifierFilename(verifierName string) (string, error) {
	if strings.HasPrefix(verifierName, "http") {
		parts := strings.Split(verifierName, "/")
		last := parts[len(parts)-1]
		if last == "" {
			return "", errors.New("snarkjs: invalid verifierName")
		}
		return last, nil
	}
	return verifierName, nil
}

func GenerateZkProofEnclave(
	ctx context.Context,
	chainID int,
	verifierNames []string,
	inputs []any,
) ([]ZkProofResult, error) {
	wasmFilenames := make([]string, len(verifierNames))
	zkeyFilenames := make([]string, len(verifierNames))
	for i, name := range verifierNames {
		wasm, err := retrieveVerifierFilename(GetWASMFile(name, chainID))
		if err != nil {
			return nil, err
		}
		zkey, err := retrieveVerifierFilename(GetZKeyFile(name, chainID))
		if err != nil {
			return nil, err
		}
		wasmFilenames[i] = wasm
		zkeyFilenames[i] = zkey
	}

	inputBytes, err := json.Marshal(toJSONSafe(inputs))
	if err != nil {
		return nil, fmt.Errorf("snarkjs: marshal enclave inputs: %w", err)
	}

	handshake, err := enclave.MakeHandshakeAndEncrypt(ctx, inputBytes)
	if err != nil {
		return nil, err
	}

	responses, err := api.GenerateProofsEnclaveCall(ctx, wasmFilenames, zkeyFilenames, handshake.InputCiphertext, handshake.KeyCiphertext)
	if err != nil {
		return nil, err
	}

	results := make([]ZkProofResult, len(responses))
	for i, r := range responses {
		results[i] = ZkProofResult{ZkCallData: r.ZkCalldata, PublicSignals: r.PublicSignals}
	}
	return results, nil
}
