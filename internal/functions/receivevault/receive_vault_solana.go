package receivevault

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	solana "github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var receiveVaultSeed = []byte("hinkal_receive_vault")

func solanaField(value *big.Int) ([]byte, error) {
	if value == nil || value.Sign() < 0 || value.BitLen() > 256 {
		return nil, fmt.Errorf("receive vault: field must fit in u256")
	}
	field := make([]byte, 32)
	value.FillBytes(field)
	return field, nil
}

func solanaVersionByte(version *big.Int) (byte, error) {
	if version == nil || version.Sign() < 0 || version.BitLen() > 8 {
		return 0, fmt.Errorf("receive vault: version must fit in u8")
	}
	return byte(version.Uint64()), nil
}

func GetSolanaReceiveVaultParamsHash(params types.ReceiveVaultParams) ([]byte, error) {
	ownerKey, err := solana.PublicKeyFromBase58(params.Owner)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana owner: %w", err)
	}
	versionByte, err := solanaVersionByte(params.Version)
	if err != nil {
		return nil, err
	}

	preimage := make([]byte, 0, 32*6+1)
	for _, value := range []*big.Int{
		params.Recipient.H0x,
		params.Recipient.H0y,
		params.Recipient.H1x,
		params.Recipient.H1y,
		params.Recipient.StealthAddress,
	} {
		field, err := solanaField(value)
		if err != nil {
			return nil, err
		}
		preimage = append(preimage, field...)
	}
	preimage = append(preimage, ownerKey.Bytes()...)
	preimage = append(preimage, versionByte)

	hash := sha256.Sum256(preimage)
	return hash[:], nil
}

func ComputeSolanaReceiveVaultAddress(params types.ReceiveVaultParams, programID, originalDeployer string) (string, error) {
	paramsHash, err := GetSolanaReceiveVaultParamsHash(params)
	if err != nil {
		return "", err
	}
	programKey, err := solana.PublicKeyFromBase58(programID)
	if err != nil {
		return "", fmt.Errorf("receive vault: invalid solana program id: %w", err)
	}
	deployerKey, err := solana.PublicKeyFromBase58(originalDeployer)
	if err != nil {
		return "", fmt.Errorf("receive vault: invalid solana original deployer: %w", err)
	}
	vaultKey, _, err := solana.FindProgramAddress([][]byte{receiveVaultSeed, deployerKey.Bytes(), paramsHash}, programKey)
	if err != nil {
		return "", fmt.Errorf("receive vault: unable to derive solana vault address: %w", err)
	}
	return vaultKey.String(), nil
}

func GetSolanaReceiveVaultOutputDigest(programID, originalDeployer, vault string, encryptedOutput []byte) ([]byte, error) {
	programKey, err := solana.PublicKeyFromBase58(programID)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana program id: %w", err)
	}
	deployerKey, err := solana.PublicKeyFromBase58(originalDeployer)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana original deployer: %w", err)
	}
	vaultKey, err := solana.PublicKeyFromBase58(vault)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana vault: %w", err)
	}
	blobHash := sha256.Sum256(encryptedOutput)

	preimage := append([]byte(outputScope), programKey.Bytes()...)
	preimage = append(preimage, deployerKey.Bytes()...)
	preimage = append(preimage, vaultKey.Bytes()...)
	preimage = append(preimage, blobHash[:]...)

	hash := sha256.Sum256(preimage)
	return hash[:], nil
}

func GetSolanaReceiveVaultRecoveryDigest(programID, originalDeployer, vault, mint, to string, deadline *big.Int) ([]byte, error) {
	if deadline == nil || deadline.Sign() < 0 || !deadline.IsInt64() {
		return nil, fmt.Errorf("receive vault: invalid deadline")
	}
	programKey, err := solana.PublicKeyFromBase58(programID)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana program id: %w", err)
	}
	deployerKey, err := solana.PublicKeyFromBase58(originalDeployer)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana original deployer: %w", err)
	}
	vaultKey, err := solana.PublicKeyFromBase58(vault)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana vault: %w", err)
	}
	mintKey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana mint: %w", err)
	}
	toKey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana recipient: %w", err)
	}

	deadlineLE := make([]byte, 8)
	binary.LittleEndian.PutUint64(deadlineLE, uint64(deadline.Int64()))

	preimage := append([]byte(recoveryScope), programKey.Bytes()...)
	preimage = append(preimage, deployerKey.Bytes()...)
	preimage = append(preimage, vaultKey.Bytes()...)
	preimage = append(preimage, mintKey.Bytes()...)
	preimage = append(preimage, toKey.Bytes()...)
	preimage = append(preimage, deadlineLE...)

	hash := sha256.Sum256(preimage)
	return hash[:], nil
}

func SignSolanaReceiveVaultDigest(owner ed25519.PrivateKey, digest []byte) string {
	return base58.Encode(ed25519.Sign(owner, digest))
}
