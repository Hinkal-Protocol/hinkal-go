package web3

import (
	"encoding/binary"
	"math/big"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func bigToBE(value *big.Int, length int) []byte {
	out := make([]byte, length)
	if value != nil {
		value.FillBytes(out)
	}
	return out
}

type AnchorStealthAddressStructure struct {
	H0X            [32]byte
	H0Y            [32]byte
	H1X            [32]byte
	H1Y            [32]byte
	StealthAddress [32]byte
}

func BuildAnchorStealthAddressStructure(s types.StealthAddressStructure) AnchorStealthAddressStructure {
	return AnchorStealthAddressStructure{
		H0X:            solanautils.EncodeToByte32Array(s.H0x),
		H0Y:            solanautils.EncodeToByte32Array(s.H0y),
		H1X:            solanautils.EncodeToByte32Array(s.H1x),
		H1Y:            solanautils.EncodeToByte32Array(s.H1y),
		StealthAddress: solanautils.EncodeToByte32Array(s.StealthAddress),
	}
}

func mustFindPDA(seeds [][]byte, programID solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(seeds, programID)
	return addr, err
}

func GetNullifierAccount(nullifier *big.Int, originalDeployer, programID solana.PublicKey) (solana.PublicKey, error) {
	nullifierBytes := solanautils.EncodeToByte32Array(nullifier)
	return mustFindPDA([][]byte{[]byte("hinkal_nullifier"), originalDeployer.Bytes(), nullifierBytes[:]}, programID)
}

func GetStorageAccountPublicKey(programID, originalDeployer solana.PublicKey) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal"), originalDeployer.Bytes()}, programID)
}

func GetMerkleAccountPublicKey(programID, originalDeployer solana.PublicKey) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_merkle"), originalDeployer.Bytes()}, programID)
}

func GetRootBucketPublicKey(programID, merkleAccount solana.PublicKey, bucketIndex *big.Int) (solana.PublicKey, error) {
	seed := make([]byte, 8)
	binary.LittleEndian.PutUint64(seed, bucketIndex.Uint64())
	return mustFindPDA([][]byte{[]byte("hinkal_root_bucket"), merkleAccount.Bytes(), seed}, programID)
}

func GetStorageVaultPublicKey(programID, originalDeployer solana.PublicKey) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_vault"), originalDeployer.Bytes()}, programID)
}

func GetSwapperAccountPublicKey(programID, originalDeployer solana.PublicKey, swapperAccountAdditionalSeed *big.Int) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_swapper"), originalDeployer.Bytes(), bigToBE(swapperAccountAdditionalSeed, 32)}, programID)
}

func GetSwapperAccountPublicKeyFromSalt(programID, originalDeployer solana.PublicKey, swapperAccountSalt *big.Int) (solana.PublicKey, error) {
	additionalSeed, err := crypto.PoseidonBig(swapperAccountSalt)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return GetSwapperAccountPublicKey(programID, originalDeployer, additionalSeed)
}

func GetProofAccountPart1PublicKey(programID, originalDeployer, signer solana.PublicKey, id *big.Int) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_proof_part1"), originalDeployer.Bytes(), signer.Bytes(), bigToBE(id, 4)}, programID)
}

func GetProofAccountPart2PublicKey(programID, originalDeployer, signer solana.PublicKey, id *big.Int) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_proof_part2"), originalDeployer.Bytes(), signer.Bytes(), bigToBE(id, 4)}, programID)
}

func GetEncryptedOutputsAccountPublicKey(programID, originalDeployer, signer solana.PublicKey, id *big.Int) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_encrypted_outputs"), originalDeployer.Bytes(), signer.Bytes(), bigToBE(id, 4)}, programID)
}

func GetInstructionsAccountPublicKey(programID, originalDeployer, signer solana.PublicKey, id *big.Int) (solana.PublicKey, error) {
	return mustFindPDA([][]byte{[]byte("hinkal_instructions"), originalDeployer.Bytes(), signer.Bytes(), bigToBE(id, 4)}, programID)
}
