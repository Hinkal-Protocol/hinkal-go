package snarkjs

import (
	"context"
	"errors"
)

var ErrSelfProvingNotImplemented = errors.New("snarkjs: local (self) proving not implemented; use the remote/enclave path")

func GenerateZkProofSelf(_ context.Context, _ int, _ string, _ any) (ZkProofResult, error) {
	return ZkProofResult{}, ErrSelfProvingNotImplemented
}
