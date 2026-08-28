package transactions

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	tokenprogram "github.com/gagliardetto/solana-go/programs/token"
	token2022 "github.com/gagliardetto/solana-go/programs/token-2022"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	hinkalsolana "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type multiPaymentDepositArgs struct {
	Amounts                  []uint64
	StealthAddressStructures []web3.AnchorStealthAddressStructure
	EncryptedBlobs           [][]byte
	CreateBlockedUtxos       bool
}

var (
	errAmountsEmpty                    = errors.New("amounts must not be empty")
	errAmountsStealthMismatch          = errors.New("amounts and stealthAddressStructures length mismatch")
	errAmountNotPositive               = errors.New("all amounts must be positive")
	errSolanaAmountTooLarge            = errors.New("transactions: Solana amount does not fit in uint64")
	errNotSolanaChain                  = errors.New("transactions: Solana deposit requires a Solana chain")
	errSolanaProoflessDepositNonSolana = errors.New("hinkalSolanaProoflessDepositHash: non-Solana chain")
	errSolanaProoflessOneMint          = errors.New("Solana prooflessDeposit supports one mint per transaction")
	multiPaymentDepositAtDiscrim       = []byte{20, 97, 177, 53, 252, 2, 24, 176}
)

const solanaBlockheightExpiredRetryCount = 10

func validateSolanaDepositArgs(amounts []*big.Int, structures []types.StealthAddressStructure) error {
	if len(amounts) == 0 {
		return errAmountsEmpty
	}
	if len(amounts) != len(structures) {
		return errAmountsStealthMismatch
	}
	for _, amount := range amounts {
		if amount.Sign() <= 0 {
			return errAmountNotPositive
		}
	}
	return assertNoDuplicateStealthAddressStructures(structures)
}

// The instruction is always multi_payment_deposit_at; timestampOverride nil encodes Anchor's
// Option::None, which makes the program fall back to the block clock.
func encodeMultiPaymentDepositData(amounts []*big.Int, structures []types.StealthAddressStructure, encryptedBlobs [][]byte, createBlockedUtxos bool, timestampOverride *int64) ([]byte, error) {
	args := multiPaymentDepositArgs{
		Amounts:                  make([]uint64, len(amounts)),
		StealthAddressStructures: make([]web3.AnchorStealthAddressStructure, len(structures)),
		EncryptedBlobs:           encryptedBlobs,
		CreateBlockedUtxos:       createBlockedUtxos,
	}
	for i, amount := range amounts {
		args.Amounts[i] = amount.Uint64()
	}
	for i, s := range structures {
		args.StealthAddressStructures[i] = web3.BuildAnchorStealthAddressStructure(s)
	}

	body, err := bin.MarshalBorsh(args)
	if err != nil {
		return nil, err
	}
	data := append(append([]byte{}, multiPaymentDepositAtDiscrim...), body...)
	if timestampOverride == nil {
		return append(data, 0), nil
	}
	data = append(data, 1)
	return binary.LittleEndian.AppendUint64(data, uint64(*timestampOverride)), nil
}

func resolveSolanaRootBuckets(
	ctx context.Context,
	connection *rpc.Client,
	programID, merkleAccount solana.PublicKey,
	insertCount int,
) (*solana.AccountMeta, *solana.AccountMeta, error) {
	info, err := connection.GetAccountInfo(ctx, merkleAccount)
	if err != nil {
		return nil, nil, err
	}
	if info == nil || info.Value == nil {
		return nil, nil, errors.New("transactions: merkle account not found")
	}
	mIndex, err := hinkalsolana.ParseMerkleMIndex(info.Value.Data.GetBinary())
	if err != nil {
		return nil, nil, err
	}
	relative := new(big.Int).Sub(mIndex, hinkalsolana.MerkleMinimumIndex())
	bucketIndex := hinkalsolana.RootBucketIndex(relative)

	current, err := web3.GetRootBucketPublicKey(programID, merkleAccount, bucketIndex)
	if err != nil {
		return nil, nil, err
	}

	slot := hinkalsolana.RootBucketSlot(relative)
	straddles := new(big.Int).Add(slot, big.NewInt(int64(insertCount))).Cmp(big.NewInt(hinkalsolana.RootBucketCap)) > 0
	if !straddles {
		return solana.NewAccountMeta(current, true, false), solana.NewAccountMeta(programID, false, false), nil
	}
	next, err := web3.GetRootBucketPublicKey(programID, merkleAccount, new(big.Int).Add(bucketIndex, big.NewInt(1)))
	if err != nil {
		return nil, nil, err
	}
	return solana.NewAccountMeta(current, true, false), solana.NewAccountMeta(next, true, false), nil
}

func buildSolanaDepositEncryptedBlobs(structures []types.StealthAddressStructure, recipientEncryptionKeys []string) ([][]byte, error) {
	outputs, err := buildProoflessEncryptedOutputs(structures, recipientEncryptionKeys)
	if err != nil {
		return nil, err
	}
	blobs := make([][]byte, len(outputs))
	for i, out := range outputs {
		blobs[i] = common.FromHex(out)
	}
	return blobs, nil
}

func buildMultiPaymentDepositInstruction(
	ctx context.Context,
	connection *rpc.Client,
	programID, signer, originalDeployer solana.PublicKey,
	mintAddress string,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	encryptedBlobs [][]byte,
	createBlockedUtxos bool,
	timestampOverride *int64,
) (*solana.GenericInstruction, error) {
	storageAccount, err := web3.GetStorageAccountPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}
	storageVault, err := web3.GetStorageVaultPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}
	merkleAccount, err := web3.GetMerkleAccountPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}

	isNative := mintAddress == constants.SolanaNativeAddress
	mintMeta := solana.NewAccountMeta(programID, false, false)
	signerAtaMeta := solana.NewAccountMeta(programID, false, false)
	storageVaultAtaMeta := solana.NewAccountMeta(programID, false, false)
	if !isNative {
		mint, err := solana.PublicKeyFromBase58(mintAddress)
		if err != nil {
			return nil, err
		}
		signerAta, _, err := solana.FindAssociatedTokenAddress(signer, mint)
		if err != nil {
			return nil, err
		}
		storageVaultAta, _, err := solana.FindAssociatedTokenAddress(storageVault, mint)
		if err != nil {
			return nil, err
		}
		mintMeta = solana.NewAccountMeta(mint, false, false)
		signerAtaMeta = solana.NewAccountMeta(signerAta, true, false)
		storageVaultAtaMeta = solana.NewAccountMeta(storageVaultAta, true, false)
	}

	rootBucketCurrent, rootBucketNext, err := resolveSolanaRootBuckets(ctx, connection, programID, merkleAccount, len(amounts))
	if err != nil {
		return nil, err
	}

	accounts := solana.AccountMetaSlice{
		solana.NewAccountMeta(signer, true, true),
		solana.NewAccountMeta(originalDeployer, false, false),
		solana.NewAccountMeta(storageAccount, true, false),
		solana.NewAccountMeta(storageVault, true, false),
		mintMeta,
		signerAtaMeta,
		storageVaultAtaMeta,
		solana.NewAccountMeta(merkleAccount, true, false),
		rootBucketCurrent,
		rootBucketNext,
		solana.NewAccountMeta(solana.TokenProgramID, false, false),
		solana.NewAccountMeta(solana.SPLAssociatedTokenAccountProgramID, false, false),
		solana.NewAccountMeta(solana.SystemProgramID, false, false),
	}

	data, err := encodeMultiPaymentDepositData(amounts, structures, encryptedBlobs, createBlockedUtxos, timestampOverride)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(programID, accounts, data), nil
}

func solanaAmountUint64(amount *big.Int) (uint64, error) {
	if amount == nil || amount.Sign() < 0 {
		return 0, errAmountNotPositive
	}
	if !amount.IsUint64() {
		return 0, errSolanaAmountTooLarge
	}
	return amount.Uint64(), nil
}

func buildSolanaTransferInstructions(
	from solana.PublicKey,
	to solana.PublicKey,
	token types.ERC20Token,
	amount *big.Int,
) ([]solana.Instruction, error) {
	value, err := solanaAmountUint64(amount)
	if err != nil {
		return nil, err
	}
	if value == 0 {
		return nil, nil
	}

	if token.Erc20TokenAddress == constants.SolanaNativeAddress {
		return []solana.Instruction{
			system.NewTransferInstruction(value, from, to).Build(),
		}, nil
	}

	mint, err := solana.PublicKeyFromBase58(token.Erc20TokenAddress)
	if err != nil {
		return nil, err
	}

	tokenProgramID := solana.TokenProgramID
	if token.Is2022Program {
		tokenProgramID = solana.Token2022ProgramID
	}
	fromATA, _, err := solana.FindAssociatedTokenAddressWithProgram(from, mint, tokenProgramID)
	if err != nil {
		return nil, err
	}
	toATA, _, err := solana.FindAssociatedTokenAddressWithProgram(to, mint, tokenProgramID)
	if err != nil {
		return nil, err
	}

	instructions := []solana.Instruction{
		associatedtokenaccount.NewCreateIdempotentInstructionWithTokenProgram(from, to, mint, tokenProgramID).Build(),
	}
	if token.Is2022Program {
		if token.Decimals < 0 || token.Decimals > 255 {
			return nil, fmt.Errorf("transactions: invalid Solana token decimals %d", token.Decimals)
		}
		instructions = append(instructions, token2022.NewTransferCheckedInstruction(
			value,
			uint8(token.Decimals),
			fromATA,
			mint,
			toATA,
			from,
			nil,
		).Build())
	} else {
		instructions = append(instructions, tokenprogram.NewTransferInstruction(value, fromATA, toATA, from, nil).Build())
	}
	return instructions, nil
}

const (
	maxComputeUnitLimit       = 1_400_000
	minComputeUnitLimit       = 200_000
	computeUnitPaddingPercent = 0.2
)

// Simulate at the ceiling, then reserve what was actually consumed plus padding. Any failure
// falls back to the ceiling, which is what the transaction would have asked for anyway.
func estimateSolanaComputeUnitLimit(
	ctx context.Context,
	connection *rpc.Client,
	payer solana.PublicKey,
	instructions []solana.Instruction,
) uint32 {
	latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return maxComputeUnitLimit
	}
	tx, err := solana.NewTransaction(withComputeUnitLimit(instructions, maxComputeUnitLimit), latest.Value.Blockhash, solana.TransactionPayer(payer))
	if err != nil {
		return maxComputeUnitLimit
	}
	simulation, err := connection.SimulateTransactionWithOpts(ctx, tx, &rpc.SimulateTransactionOpts{
		ReplaceRecentBlockhash: true,
		Commitment:             rpc.CommitmentConfirmed,
	})
	if err != nil || simulation == nil || simulation.Value == nil {
		return maxComputeUnitLimit
	}
	if simulation.Value.Err != nil {
		log.Printf("solana compute-unit simulation failed: %v", simulation.Value.Err)
	}
	if simulation.Value.UnitsConsumed == nil || *simulation.Value.UnitsConsumed == 0 {
		return maxComputeUnitLimit
	}
	padded := uint64(math.Ceil(float64(*simulation.Value.UnitsConsumed) * (1 + computeUnitPaddingPercent)))
	return uint32(max(minComputeUnitLimit, min(padded, maxComputeUnitLimit)))
}

func withComputeUnitLimit(instructions []solana.Instruction, limit uint32) []solana.Instruction {
	budget := []solana.Instruction{computebudget.NewSetComputeUnitLimitInstruction(limit).Build()}
	return append(budget, instructions...)
}

func withSolanaComputeBudget(ctx context.Context, connection *rpc.Client, payer solana.PublicKey, instructions []solana.Instruction) []solana.Instruction {
	return withComputeUnitLimit(instructions, estimateSolanaComputeUnitLimit(ctx, connection, payer, instructions))
}

func buildSolanaDepositTransaction(
	ctx context.Context,
	connection *rpc.Client,
	signer solana.PublicKey,
	instructions []solana.Instruction,
) (*solana.Transaction, error) {
	latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return nil, err
	}
	return solana.NewTransaction(
		withSolanaComputeBudget(ctx, connection, signer, instructions),
		latest.Value.Blockhash,
		solana.TransactionPayer(signer),
	)
}

func shouldRetrySolanaSendError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "blockhash not found") ||
		strings.Contains(msg, "blockhashnotfound") ||
		strings.Contains(msg, "transactionexpiredblockheightexceedederror") ||
		strings.Contains(msg, "block height exceeded")
}

const solanaConfirmationPollInterval = 500 * time.Millisecond

var errSolanaConfirmationTimeout = errors.New("transactions: Solana transaction confirmation timed out")

func waitForSolanaConfirmation(ctx context.Context, connection *rpc.Client, signature solana.Signature, lastValidBlockHeight uint64) error {
	for {
		height, err := connection.GetBlockHeight(ctx, rpc.CommitmentConfirmed)
		if err == nil && height > lastValidBlockHeight {
			return errSolanaConfirmationTimeout
		}

		statuses, err := connection.GetSignatureStatuses(ctx, false, signature)
		if err == nil && len(statuses.Value) > 0 && statuses.Value[0] != nil {
			status := statuses.Value[0]
			if status.Err != nil {
				return fmt.Errorf("transactions: Solana transaction failed: %v", status.Err)
			}
			if status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed || status.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(solanaConfirmationPollInterval):
		}
	}
}

func signAndSendSolanaInstructions(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	programID solana.PublicKey,
	connection *rpc.Client,
	signer solana.PublicKey,
	instructions []solana.Instruction,
) (string, error) {
	program, err := hinkal.GetSolanaProgram(programID)
	if err != nil {
		return "", err
	}

	for i := 0; i < solanaBlockheightExpiredRetryCount; i++ {
		latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
		if err != nil {
			return "", err
		}
		tx, err := solana.NewTransaction(withSolanaComputeBudget(ctx, connection, signer, instructions), latest.Value.Blockhash, solana.TransactionPayer(signer))
		if err != nil {
			return "", err
		}

		signature, err := program.SignAndSend(ctx, tx)
		if err == nil {
			if confirmErr := waitForSolanaConfirmation(ctx, connection, signature, latest.Value.LastValidBlockHeight); confirmErr != nil {
				if i == solanaBlockheightExpiredRetryCount-1 || !shouldRetrySolanaSendError(confirmErr) {
					return "", confirmErr
				}
				continue
			}
			return signature.String(), nil
		}
		if i == solanaBlockheightExpiredRetryCount-1 || !shouldRetrySolanaSendError(err) {
			return "", err
		}
	}
	return "", errors.New("transactions: Solana deposit failed")
}

// sendSolanaInstructions signs and broadcasts without waiting for confirmation, so a dependent
// proof can be built while the transaction is still landing.
func sendSolanaInstructions(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	programID solana.PublicKey,
	connection *rpc.Client,
	signer solana.PublicKey,
	instructions []solana.Instruction,
) (string, error) {
	program, err := hinkal.GetSolanaProgram(programID)
	if err != nil {
		return "", err
	}
	latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return "", err
	}
	tx, err := solana.NewTransaction(withSolanaComputeBudget(ctx, connection, signer, instructions), latest.Value.Blockhash, solana.TransactionPayer(signer))
	if err != nil {
		return "", err
	}
	signature, err := program.SignAndSend(ctx, tx)
	if err != nil {
		return "", err
	}
	return signature.String(), nil
}

func SubmitSolanaProoflessDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	recipientEncryptionKeys []string,
	returnTxData bool,
) (string, error) {
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return "", err
	}
	chainID, err := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return "", err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return "", err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return "", err
	}
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return "", err
	}

	encryptedBlobs, err := buildSolanaDepositEncryptedBlobs(structures, recipientEncryptionKeys)
	if err != nil {
		return "", err
	}

	instruction, err := buildMultiPaymentDepositInstruction(ctx, connection, programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, encryptedBlobs, false, nil)
	if err != nil {
		return "", err
	}

	if returnTxData {
		tx, err := buildSolanaDepositTransaction(ctx, connection, signer, []solana.Instruction{instruction})
		if err != nil {
			return "", err
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(raw), nil
	}

	return signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, []solana.Instruction{instruction})
}

func HinkalSolanaProoflessDepositWithPublicFee(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	recipientEncryptionKeys []string,
	feeAmount *big.Int,
) (string, error) {
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return "", err
	}
	chainID, err := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return "", err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return "", err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return "", err
	}
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return "", err
	}
	feeRecipientStr, err := hinkal.GetRandomRelay(ctx, chainID, false)
	if err != nil {
		return "", err
	}
	if feeRecipientStr == "" {
		return "", fmt.Errorf("no relay available for chainId %d", chainID)
	}
	feeRecipient, err := solana.PublicKeyFromBase58(feeRecipientStr)
	if err != nil {
		return "", err
	}

	feeInstructions, err := buildSolanaTransferInstructions(signer, feeRecipient, token, feeAmount)
	if err != nil {
		return "", err
	}
	encryptedBlobs, err := buildSolanaDepositEncryptedBlobs(structures, recipientEncryptionKeys)
	if err != nil {
		return "", err
	}

	depositInstruction, err := buildMultiPaymentDepositInstruction(ctx, connection, programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, encryptedBlobs, false, nil)
	if err != nil {
		return "", err
	}
	instructions := make([]solana.Instruction, 0, len(feeInstructions)+1)
	instructions = append(instructions, feeInstructions...)
	instructions = append(instructions, depositInstruction)
	signature, err := signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, instructions)
	if err != nil {
		return "", err
	}
	emitDepositAdminData(ctx, hinkal, types.AdminShield, chainID, []types.ERC20Token{token}, amounts)
	return signature, nil
}

func HinkalSolanaProoflessDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	stealthAddressStructuresOverride []types.StealthAddressStructure,
	recipientEncryptionKeysOverride []string,
	returnTxData bool,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errSolanaProoflessDepositNonSolana
	}
	if len(erc20Tokens) != len(amountChanges) {
		return "", errTokenAmountLengthMismatch
	}

	firstToken := erc20Tokens[0]
	for _, token := range erc20Tokens {
		if token.Erc20TokenAddress != firstToken.Erc20TokenAddress {
			return "", errSolanaProoflessOneMint
		}
	}

	if stealthAddressStructuresOverride != nil && recipientEncryptionKeysOverride == nil {
		return "", errRecipientEncryptionKeysRequired
	}
	if recipientEncryptionKeysOverride != nil && len(recipientEncryptionKeysOverride) != len(amountChanges) {
		return "", errRecipientEncryptionKeysLengthMismatch
	}

	stealthAddressStructures, err := getProoflessStealthAddressStructures(hinkal, len(amountChanges), stealthAddressStructuresOverride)
	if err != nil {
		return "", err
	}
	recipientEncryptionKeys := recipientEncryptionKeysOverride
	if recipientEncryptionKeys == nil {
		recipientEncryptionKeys, err = getOwnRecipientEncryptionKeys(hinkal, len(amountChanges))
		if err != nil {
			return "", err
		}
	}

	result, err := SubmitSolanaProoflessDeposit(ctx, hinkal, firstToken, amountChanges, stealthAddressStructures, recipientEncryptionKeys, returnTxData)
	if err != nil {
		return "", err
	}
	if !returnTxData {
		emitDepositAdminData(ctx, hinkal, types.AdminShield, chainID, erc20Tokens, amountChanges)
	}
	return result, nil
}

func HinkalSolanaDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	amount *big.Int,
	token types.ERC20Token,
	returnTxData bool,
) (string, error) {
	userKeys := hinkal.GetUserKeys()
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
	if err != nil {
		return "", err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return "", err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	structure, err := snarkjs.CalcStealthAddressStructure(extraRandomization, shieldedPrivateKey, spendingPublicKey)
	if err != nil {
		return "", err
	}
	ownKeys, err := getOwnRecipientEncryptionKeys(hinkal, 1)
	if err != nil {
		return "", err
	}
	return SubmitSolanaProoflessDeposit(ctx, hinkal, token, []*big.Int{amount}, []types.StealthAddressStructure{structure}, ownKeys, returnTxData)
}

func HinkalSolanaDepositForOther(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	amount *big.Int,
	token types.ERC20Token,
	recipientInfo string,
	returnTxData bool,
) (string, error) {
	if !pretransaction.IsValidPrivateAddress(recipientInfo) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}
	structure, err := pretransaction.ConstructStealthAddressStructure(recipientInfo)
	if err != nil {
		return "", err
	}
	encryptionKey, err := pretransaction.GetEncryptionKeyFromRecipientInfo(recipientInfo)
	if err != nil {
		return "", err
	}
	result, err := SubmitSolanaProoflessDeposit(ctx, hinkal, token, []*big.Int{amount}, []types.StealthAddressStructure{structure}, []string{encryptionKey}, returnTxData)
	if err != nil {
		return "", err
	}
	if !returnTxData {
		chainID, chainErr := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
		if chainErr == nil {
			emitDepositAdminData(ctx, hinkal, types.AdminShield, chainID, []types.ERC20Token{token}, []*big.Int{amount})
		}
	}
	return result, nil
}
