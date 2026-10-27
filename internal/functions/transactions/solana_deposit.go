package transactions

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
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
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type prooflessDepositArgs struct {
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
	prooflessDepositDiscrim            = []byte{238, 63, 30, 216, 109, 36, 109, 123}
)

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

func buildProoflessEncryptedBlobs(structures []types.StealthAddressStructure, recipientEncryptionKeys []string) ([][]byte, error) {
	outputs, err := buildProoflessEncryptedOutputs(structures, recipientEncryptionKeys)
	if err != nil {
		return nil, err
	}
	blobs := make([][]byte, len(outputs))
	for i, output := range outputs {
		blobs[i] = common.FromHex(output)
	}
	return blobs, nil
}

func encodeProoflessDepositData(amounts []*big.Int, structures []types.StealthAddressStructure, encryptedBlobs [][]byte, createBlockedUtxos bool, timestampOverride *int64) ([]byte, error) {
	args := prooflessDepositArgs{
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
	data := append(append([]byte{}, prooflessDepositDiscrim...), body...)
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

func buildProoflessDepositInstruction(
	ctx context.Context,
	connection *rpc.Client,
	programID, signer, originalDeployer solana.PublicKey,
	mintAddress string,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	recipientEncryptionKeys []string,
	createBlockedUtxos bool,
	timestampOverride *int64,
) (*solana.GenericInstruction, error) {
	encryptedBlobs, err := buildProoflessEncryptedBlobs(structures, recipientEncryptionKeys)
	if err != nil {
		return nil, err
	}

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

	data, err := encodeProoflessDepositData(amounts, structures, encryptedBlobs, createBlockedUtxos, timestampOverride)
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
	return solanautils.NewV1Transaction(ctx, connection, signer, instructions, latest.Value.Blockhash)
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

	return solanautils.SignAndConfirm(ctx, connection, program, func(blockhash solana.Hash) (*solana.Transaction, error) {
		return solanautils.NewV1Transaction(ctx, connection, signer, instructions, blockhash)
	})
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
	tx, err := buildSolanaDepositTransaction(ctx, connection, signer, instructions)
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

	instruction, err := buildProoflessDepositInstruction(ctx, connection, programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, recipientEncryptionKeys, false, nil)
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
	returnTxData bool,
	action types.AdminTransactionType,
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
	depositInstruction, err := buildProoflessDepositInstruction(ctx, connection, programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, recipientEncryptionKeys, false, nil)
	if err != nil {
		return "", err
	}
	instructions := make([]solana.Instruction, 0, len(feeInstructions)+1)
	instructions = append(instructions, feeInstructions...)
	instructions = append(instructions, depositInstruction)

	if returnTxData {
		tx, err := buildSolanaDepositTransaction(ctx, connection, signer, instructions)
		if err != nil {
			return "", err
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(raw), nil
	}

	signature, err := signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, instructions)
	if err != nil {
		return "", err
	}
	emitDepositAdminData(ctx, hinkal, action, chainID, []types.ERC20Token{token}, amounts, nil, signature)
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
	action types.AdminTransactionType,
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
		emitDepositAdminData(ctx, hinkal, action, chainID, erc20Tokens, amountChanges, nil, result)
	}
	return result, nil
}

func HinkalSolanaDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	amount *big.Int,
	token types.ERC20Token,
	returnTxData bool,
	action types.AdminTransactionType,
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
	encryptionKeys, err := getOwnRecipientEncryptionKeys(hinkal, 1)
	if err != nil {
		return "", err
	}
	result, err := SubmitSolanaProoflessDeposit(ctx, hinkal, token, []*big.Int{amount}, []types.StealthAddressStructure{structure}, encryptionKeys, returnTxData)
	if err != nil {
		return "", err
	}
	if !returnTxData {
		emitDepositAdminData(ctx, hinkal, action, token.ChainID, []types.ERC20Token{token}, []*big.Int{amount}, nil, result)
	}
	return result, nil
}

func HinkalSolanaDepositForOther(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	amount *big.Int,
	token types.ERC20Token,
	recipientInfo string,
	returnTxData bool,
	variableRate *big.Int,
	action types.AdminTransactionType,
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

	feeAmount := big.NewInt(0)
	if variableRate != nil && variableRate.Sign() > 0 {
		feeAmount = new(big.Int).Mul(amount, variableRate)
		feeAmount.Div(feeAmount, big.NewInt(constants.BpsDenominator))
	}

	result, err := HinkalSolanaProoflessDepositWithPublicFee(ctx, hinkal, token, []*big.Int{amount}, []types.StealthAddressStructure{structure}, []string{encryptionKey}, feeAmount, returnTxData, "")
	if err != nil {
		return "", err
	}
	if !returnTxData {
		chainID, chainErr := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
		if chainErr == nil {
			emitDepositAdminData(ctx, hinkal, action, chainID, []types.ERC20Token{token}, []*big.Int{amount}, variableRate, result)
		}
	}
	return result, nil
}
