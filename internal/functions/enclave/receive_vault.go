package enclave

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type registerReceiveVaultRequest struct {
	VaultAddress     string                              `json:"vaultAddress"`
	Network          types.ReceiveVaultNetwork           `json:"network"`
	Index            int                                 `json:"index,string"`
	DeploymentMode   string                              `json:"deploymentMode"`
	Params           types.ReceiveVaultParamsJSON        `json:"params"`
	EncryptionKey    string                              `json:"encryptionKey"`
	EncryptedOutput  string                              `json:"encryptedOutput"`
	OutputSignatures []types.ReceiveVaultOutputSignature `json:"outputSignatures"`
	AccountKey       string                              `json:"accountKey"`
}

type registerReceiveVaultResponse struct {
	Success bool `json:"success"`
	Settled bool `json:"settled"`
}

type ReceiveVaultAccountResponse struct {
	Records []types.ReceiveVaultRecord `json:"records"`
	Blocks  []types.ReceiveVaultBlock  `json:"blocks"`
}

// RegisterReceiveVault hands the relayer everything it needs to settle the address. It reports
// whether the address was already settled, in which case its index is spent and a fresh one is due.
// Sealed, not plain: the body ties a shielded account to the address it is handing out.
func RegisterReceiveVault(
	ctx context.Context,
	record types.ReceiveVaultRecord,
	params types.ReceiveVaultParamsJSON,
	encryptionKey, accountKey string,
	encryptedOutput string,
	outputSignatures []types.ReceiveVaultOutputSignature,
) (bool, error) {
	resp, err := enclavePostSealedSigned[registerReceiveVaultResponse](
		ctx,
		constants.ServerConfig.RegisterReceiveVault,
		registerReceiveVaultRequest{
			VaultAddress:     record.VaultAddress,
			Network:          record.Network,
			Index:            record.Index,
			DeploymentMode:   string(constants.Mode),
			Params:           params,
			EncryptionKey:    encryptionKey,
			EncryptedOutput:  encryptedOutput,
			OutputSignatures: outputSignatures,
			AccountKey:       accountKey,
		},
	)
	if err != nil {
		return false, err
	}
	return resp.Settled, nil
}

// FetchReceiveVaultAccount reads back the terms of every address on the account, along with the
// balances the relayer refused to shield. The terms live in a vault's CREATE2 salt, so a client that
// lost them cannot re-derive the address to recover from it.
func FetchReceiveVaultAccount(
	ctx context.Context,
	proof types.ReceiveVaultAccountProof,
) (ReceiveVaultAccountResponse, error) {
	request := struct {
		types.ReceiveVaultAccountProof
		DeploymentMode string `json:"deploymentMode"`
	}{ReceiveVaultAccountProof: proof, DeploymentMode: string(constants.Mode)}

	return enclavePostSealedSigned[ReceiveVaultAccountResponse](
		ctx,
		constants.ServerConfig.ReceiveVaultAccount,
		request,
	)
}

func FetchReceiveVaultNextIndex(
	ctx context.Context,
	proof types.ReceiveVaultAccountProof,
	network types.ReceiveVaultNetwork,
) (int, error) {
	request := struct {
		types.ReceiveVaultAccountProof
		Network types.ReceiveVaultNetwork `json:"network"`
	}{ReceiveVaultAccountProof: proof, Network: network}

	resp, err := enclavePostSealedSigned[struct {
		Index int `json:"index"`
	}](ctx, constants.ServerConfig.ReceiveVaultNextIndex, request)
	if err != nil {
		return 0, err
	}
	return resp.Index, nil
}
