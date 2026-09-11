package receivevault

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	ed25519ProgramID                 = solana.MustPublicKeyFromBase58("Ed25519SigVerify111111111111111111111111111")
	receiveVaultRecoveryMarkerSeed   = []byte("hinkal_receive_recovery")
	receiveVaultRecoverDiscriminator = []byte{176, 251, 103, 198, 212, 208, 196, 90}
)

func encodeEd25519Instruction(owner solana.PublicKey, digest []byte, signature string) (*solana.GenericInstruction, error) {
	if len(digest) != 32 {
		return nil, errors.New("receive vault: recovery digest must be 32 bytes")
	}
	signatureBytes, err := base58.Decode(signature)
	if err != nil || len(signatureBytes) != 64 {
		return nil, errors.New("receive vault: invalid recovery signature")
	}

	data := make([]byte, 16, 144)
	data[0] = 1
	binary.LittleEndian.PutUint16(data[2:4], 48)
	binary.LittleEndian.PutUint16(data[4:6], 0xffff)
	binary.LittleEndian.PutUint16(data[6:8], 16)
	binary.LittleEndian.PutUint16(data[8:10], 0xffff)
	binary.LittleEndian.PutUint16(data[10:12], 112)
	binary.LittleEndian.PutUint16(data[12:14], uint16(len(digest)))
	binary.LittleEndian.PutUint16(data[14:16], 0xffff)
	data = append(data, owner.Bytes()...)
	data = append(data, signatureBytes...)
	data = append(data, digest...)
	return solana.NewInstruction(ed25519ProgramID, nil, data), nil
}

func encodeSolanaReceiveVaultRecoverData(
	params types.ReceiveVaultParams,
	to solana.PublicKey,
	deadline *big.Int,
	digest []byte,
) ([]byte, error) {
	if deadline == nil || deadline.Sign() < 0 || !deadline.IsInt64() {
		return nil, errors.New("receive vault: invalid deadline")
	}
	if len(digest) != 32 {
		return nil, errors.New("receive vault: recovery digest must be 32 bytes")
	}
	owner, err := solana.PublicKeyFromBase58(params.Owner)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana owner: %w", err)
	}
	version, err := solanaVersionByte(params.Version)
	if err != nil {
		return nil, err
	}

	data := append([]byte{}, receiveVaultRecoverDiscriminator...)
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
		data = append(data, field...)
	}
	data = append(data, owner.Bytes()...)
	data = append(data, version)
	data = append(data, to.Bytes()...)
	data = binary.LittleEndian.AppendUint64(data, uint64(deadline.Int64()))
	return append(data, digest...), nil
}

func buildSolanaReceiveVaultRecoverInstructions(
	params types.ReceiveVaultParams,
	token, to string,
	deadline *big.Int,
	signature string,
	programID, originalDeployer, feePayer, tokenProgram solana.PublicKey,
) ([]solana.Instruction, error) {
	if !tokenProgram.Equals(solana.TokenProgramID) && !tokenProgram.Equals(solana.Token2022ProgramID) {
		return nil, errors.New("receive vault: unsupported token program")
	}
	if token == constants.SolanaNativeAddress && !tokenProgram.Equals(solana.TokenProgramID) {
		return nil, errors.New("receive vault: unsupported token program")
	}
	vaultAddress, err := ComputeSolanaReceiveVaultAddress(params, programID.String(), originalDeployer.String())
	if err != nil {
		return nil, err
	}
	vault := solana.MustPublicKeyFromBase58(vaultAddress)
	mint, err := solana.PublicKeyFromBase58(token)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana mint: %w", err)
	}
	destination, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return nil, fmt.Errorf("receive vault: invalid solana recipient: %w", err)
	}
	digest, err := GetSolanaReceiveVaultRecoveryDigest(
		programID.String(),
		originalDeployer.String(),
		vaultAddress,
		token,
		to,
		deadline,
	)
	if err != nil {
		return nil, err
	}
	recoveryMarker, _, err := solana.FindProgramAddress(
		[][]byte{receiveVaultRecoveryMarkerSeed, originalDeployer.Bytes(), digest},
		programID,
	)
	if err != nil {
		return nil, fmt.Errorf("receive vault: unable to derive recovery marker: %w", err)
	}

	verifyInstruction, err := encodeEd25519Instruction(solana.MustPublicKeyFromBase58(params.Owner), digest, signature)
	if err != nil {
		return nil, err
	}
	recoverData, err := encodeSolanaReceiveVaultRecoverData(params, destination, deadline, digest)
	if err != nil {
		return nil, err
	}

	optionalAccount := solana.NewAccountMeta(programID, false, false)
	vaultATA := optionalAccount
	mintMeta := optionalAccount
	destinationATA := optionalAccount
	if token != constants.SolanaNativeAddress {
		vaultATAAddress, _, err := solana.FindAssociatedTokenAddressWithProgram(vault, mint, tokenProgram)
		if err != nil {
			return nil, err
		}
		destinationATAAddress, _, err := solana.FindAssociatedTokenAddressWithProgram(destination, mint, tokenProgram)
		if err != nil {
			return nil, err
		}
		vaultATA = solana.NewAccountMeta(vaultATAAddress, true, false)
		mintMeta = solana.NewAccountMeta(mint, false, false)
		destinationATA = solana.NewAccountMeta(destinationATAAddress, true, false)
	}

	accounts := solana.AccountMetaSlice{
		solana.NewAccountMeta(feePayer, true, true),
		solana.NewAccountMeta(originalDeployer, false, false),
		solana.NewAccountMeta(vault, true, false),
		vaultATA,
		mintMeta,
		solana.NewAccountMeta(destination, true, false),
		destinationATA,
		solana.NewAccountMeta(recoveryMarker, true, false),
		solana.NewAccountMeta(solana.SysVarInstructionsPubkey, false, false),
		solana.NewAccountMeta(tokenProgram, false, false),
		solana.NewAccountMeta(solana.SPLAssociatedTokenAccountProgramID, false, false),
		solana.NewAccountMeta(solana.SystemProgramID, false, false),
	}
	recoverInstruction := solana.NewInstruction(programID, accounts, recoverData)
	return []solana.Instruction{verifyInstruction, recoverInstruction}, nil
}

func resolveSolanaTokenProgram(ctx context.Context, connection *rpc.Client, token string) (solana.PublicKey, error) {
	if token == constants.SolanaNativeAddress {
		return solana.TokenProgramID, nil
	}
	mint, err := solana.PublicKeyFromBase58(token)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("receive vault: invalid solana mint: %w", err)
	}
	info, err := connection.GetAccountInfo(ctx, mint)
	if err != nil {
		if errors.Is(err, rpc.ErrNotFound) {
			return solana.PublicKey{}, errors.New("receive vault: unsupported token program")
		}
		return solana.PublicKey{}, err
	}
	if info == nil || info.Value == nil ||
		(!info.Value.Owner.Equals(solana.TokenProgramID) && !info.Value.Owner.Equals(solana.Token2022ProgramID)) {
		return solana.PublicKey{}, errors.New("receive vault: unsupported token program")
	}
	return info.Value.Owner, nil
}

func recoverSolanaReceiveVault(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	params types.ReceiveVaultParams,
	token, to string,
	deadline *big.Int,
	signature, programAddress, originalDeployerAddress string,
	returnTxData bool,
) (string, error) {
	programID, err := solana.PublicKeyFromBase58(programAddress)
	if err != nil {
		return "", fmt.Errorf("receive vault: invalid solana program id: %w", err)
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerAddress)
	if err != nil {
		return "", fmt.Errorf("receive vault: invalid solana original deployer: %w", err)
	}
	feePayer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return "", err
	}
	tokenProgram, err := resolveSolanaTokenProgram(ctx, connection, token)
	if err != nil {
		return "", err
	}
	instructions, err := buildSolanaReceiveVaultRecoverInstructions(
		params,
		token,
		to,
		deadline,
		signature,
		programID,
		originalDeployer,
		feePayer,
		tokenProgram,
	)
	if err != nil {
		return "", err
	}
	if returnTxData {
		latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
		if err != nil {
			return "", err
		}
		tx, err := solana.NewTransaction(instructions, latest.Value.Blockhash, solana.TransactionPayer(feePayer))
		if err != nil {
			return "", err
		}
		return tx.ToBase64()
	}
	program, err := hinkal.GetSolanaProgram(programID)
	if err != nil {
		return "", err
	}
	return solanautils.SignAndConfirm(ctx, connection, program, func(blockhash solana.Hash) (*solana.Transaction, error) {
		return solana.NewTransaction(instructions, blockhash, solana.TransactionPayer(feePayer))
	})
}
