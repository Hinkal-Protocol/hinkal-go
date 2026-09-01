package enclave

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// Recomputes everything behind SignedMessageHash from the caller's own data instead of trusting the
// enclave - mirrors constructTxEnclave.ts. Self-contained rather than importing snarkjs (that would
// cycle, and its CreateCallDataHash/ComputeSignedMessageHashEvm Tron branches predate henclave anyway).

func mustABIType(name string, components []abi.ArgumentMarshaling) abi.Type {
	t, err := abi.NewType(name, "", components)
	if err != nil {
		panic(err)
	}
	return t
}

var (
	verifyAbiUint16     = mustABIType("uint16", nil)
	verifyAbiUint256    = mustABIType("uint256", nil)
	verifyAbiAddress    = mustABIType("address", nil)
	verifyAbiBytes      = mustABIType("bytes", nil)
	verifyAbiBytesArr2  = mustABIType("bytes[][]", nil)
	verifyAbiBoolArr    = mustABIType("bool[]", nil)
	verifyAbiBool       = mustABIType("bool", nil)
	verifyAbiInt256Arr  = mustABIType("int256[]", nil)
	verifyAbiUint256Arr = mustABIType("uint256[]", nil)

	verifyAbiExternalActionTuple = mustABIType("tuple", []abi.ArgumentMarshaling{
		{Name: "externalAddress", Type: "address"},
		{Name: "externalActionId", Type: "uint256"},
		{Name: "externalActionMetadata", Type: "bytes"},
	})
	verifyAbiHookTuple = mustABIType("tuple", []abi.ArgumentMarshaling{
		{Name: "preHookContract", Type: "address"},
		{Name: "postHookContract", Type: "address"},
		{Name: "preHookMetadata", Type: "bytes"},
		{Name: "postHookMetadata", Type: "bytes"},
	})
	verifyAbiFeeStructureTuple = mustABIType("tuple", []abi.ArgumentMarshaling{
		{Name: "feeToken", Type: "address"},
		{Name: "flatFee", Type: "uint256"},
		{Name: "variableRate", Type: "uint256"},
	})
)

type verifyExternalActionTuple struct {
	ExternalAddress        common.Address
	ExternalActionId       *big.Int
	ExternalActionMetadata []byte
}

type verifyHookDataTuple struct {
	PreHookContract  common.Address
	PostHookContract common.Address
	PreHookMetadata  []byte
	PostHookMetadata []byte
}

type verifyFeeStructureTuple struct {
	FeeToken     common.Address
	FlatFee      *big.Int
	VariableRate *big.Int
}

func stringsToBigInts(values []string) ([]*big.Int, error) {
	out := make([]*big.Int, len(values))
	for i, v := range values {
		n, err := utils.ParseBigInt(v)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func verifyFlatToBigInts(values [][]string) ([]*big.Int, error) {
	out := make([]*big.Int, 0)
	for _, inner := range values {
		for _, v := range inner {
			n, err := utils.ParseBigInt(v)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
	}
	return out, nil
}

func verifyWrapAmountChange(amount *big.Int) *big.Int {
	if amount.Sign() < 0 {
		return new(big.Int).Add(crypto.FieldP, amount)
	}
	return new(big.Int).Set(amount)
}

func expandBoolPerToken(values []bool, count int) []bool {
	out := make([]bool, count)
	for i := 0; i < count && i < len(values); i++ {
		out[i] = values[i]
	}
	return out
}

func anyTrue(values []bool) bool {
	for _, v := range values {
		if v {
			return true
		}
	}
	return false
}

func externalAddressOrZero(externalAddress string) string {
	if externalAddress == "" {
		return constants.ZeroAddress
	}
	return externalAddress
}

func verifyGetSlippageValues(amountChanges []*big.Int) []*big.Int {
	out := make([]*big.Int, len(amountChanges))
	for i, a := range amountChanges {
		if a.Sign() >= 0 {
			out[i] = big.NewInt(0)
		} else {
			out[i] = new(big.Int).Set(a)
		}
	}
	return out
}

// Empty ID means no external action.
func verifyGetExternalActionIDHash(externalActionID string) *big.Int {
	if externalActionID == "" {
		return big.NewInt(0)
	}
	hash := new(big.Int).SetBytes(gethcrypto.Keccak256([]byte(externalActionID)))
	return hash.Mod(hash, crypto.FieldP)
}

func verifyGetOriginalSender(externalAddress, relay string) string {
	if relay == constants.ZeroAddress {
		if externalAddress == "" {
			return constants.ZeroAddress
		}
		return externalAddress
	}
	return constants.ZeroAddress
}

type verifySolanaSignedMessageHashParams struct {
	RootHashHinkal               *big.Int
	MintAccountPart1             []*big.Int
	MintAccountPart2             []*big.Int
	AmountChanges                []*big.Int
	OutTimeStamp                 *big.Int
	InNullifiers                 [][]string
	OutCommitments               [][]string
	CalldataHash                 *big.Int
	Message                      *big.Int
	SwapperAccountAdditionalSeed *big.Int
	OutH1Ay                      *big.Int
	H0Ax                         *big.Int
	H0Ay                         *big.Int
}

func verifyAppendBytes32(dst []byte, value *big.Int) []byte {
	var out [32]byte
	if value != nil {
		value.FillBytes(out[:])
	}
	return append(dst, out[:]...)
}

func verifyAppendLengthPrefixedBytes32Array(dst []byte, values []*big.Int) []byte {
	var length [8]byte
	big.NewInt(int64(len(values))).FillBytes(length[:])
	dst = append(dst, length[:]...)
	for _, value := range values {
		dst = verifyAppendBytes32(dst, value)
	}
	return dst
}

func verifyComputeSignedMessageHashSolana(params verifySolanaSignedMessageHashParams) (*big.Int, error) {
	nullifiers, err := verifyFlatToBigInts(params.InNullifiers)
	if err != nil {
		return nil, err
	}
	commitments, err := verifyFlatToBigInts(params.OutCommitments)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, 0)
	bytes = verifyAppendBytes32(bytes, params.RootHashHinkal)
	bytes = verifyAppendLengthPrefixedBytes32Array(bytes, params.MintAccountPart1)
	bytes = verifyAppendLengthPrefixedBytes32Array(bytes, params.MintAccountPart2)
	bytes = verifyAppendLengthPrefixedBytes32Array(bytes, params.AmountChanges)
	bytes = verifyAppendBytes32(bytes, params.OutTimeStamp)
	bytes = verifyAppendLengthPrefixedBytes32Array(bytes, nullifiers)
	bytes = verifyAppendLengthPrefixedBytes32Array(bytes, commitments)
	bytes = verifyAppendBytes32(bytes, params.CalldataHash)
	bytes = verifyAppendBytes32(bytes, params.Message)
	bytes = verifyAppendBytes32(bytes, params.SwapperAccountAdditionalSeed)
	bytes = verifyAppendBytes32(bytes, params.OutH1Ay)
	bytes = verifyAppendBytes32(bytes, params.H0Ax)
	bytes = verifyAppendBytes32(bytes, params.H0Ay)

	sum := sha256.Sum256(bytes)
	h := new(big.Int).SetBytes(sum[:])
	return h.Mod(h, crypto.FieldP), nil
}

// nil means "no speculative params", not "authorize nothing".
func verifyCollectSpeculativeInputCommitments(speculative *types.SpeculativeTreeParams) ([]string, error) {
	if speculative == nil {
		return nil, nil
	}
	commitments := []string{}
	for _, row := range speculative.InputUtxos {
		for _, u := range row {
			amount, err := utils.ParseBigInt(u.Amount)
			if err != nil {
				return nil, err
			}
			if amount.Sign() == 0 {
				continue
			}
			h0x, err := utils.ParseBigInt(u.H0[0])
			if err != nil {
				return nil, err
			}
			h0y, err := utils.ParseBigInt(u.H0[1])
			if err != nil {
				return nil, err
			}
			note, err := utxo.NewUtxo(types.UtxoParams{
				Amount:            amount,
				Erc20TokenAddress: u.Erc20TokenAddress,
				StealthAddress:    u.StealthAddress,
				H0:                &types.JubPoint{h0x, h0y},
				TimeStamp:         u.TimeStamp,
			})
			if err != nil {
				return nil, err
			}
			commitment, err := note.GetCommitment()
			if err != nil {
				return nil, err
			}
			commitments = append(commitments, commitment)
		}
	}
	return commitments, nil
}

// When inputCommitments is non-nil, the resulting spend set must exactly match it.
func verifyComputeInNullifiers(
	erc20Addresses []string,
	inAmounts, inH0Ax, inH0Ay, inTimeStamps [][]string,
	nullifyingKey string,
	spendingPublicKey [2]string,
	inputCommitments []string,
) ([][]string, error) {
	if len(inAmounts) != len(erc20Addresses) || len(inH0Ax) != len(erc20Addresses) ||
		len(inH0Ay) != len(erc20Addresses) || len(inTimeStamps) != len(erc20Addresses) {
		return nil, errors.New("enclave: response in-note arrays do not match the number of tokens - refusing to sign")
	}
	ownSpendingPublicKey, err := stringsToBigInts(spendingPublicKey[:])
	if err != nil {
		return nil, err
	}
	spentCommitments := map[string]struct{}{}

	inNullifiers := make([][]string, len(erc20Addresses))
	for tokenIndex, erc20TokenAddress := range erc20Addresses {
		if len(inH0Ax[tokenIndex]) != len(inAmounts[tokenIndex]) || len(inH0Ay[tokenIndex]) != len(inAmounts[tokenIndex]) ||
			len(inTimeStamps[tokenIndex]) != len(inAmounts[tokenIndex]) {
			return nil, errors.New("enclave: response in-note row lengths do not match - refusing to sign")
		}
		row := make([]string, len(inAmounts[tokenIndex]))
		for slot, amountStr := range inAmounts[tokenIndex] {
			amount, err := utils.ParseBigInt(amountStr)
			if err != nil {
				return nil, err
			}
			if amount.Sign() == 0 {
				row[slot] = "0"
				continue
			}
			h0x, err := utils.ParseBigInt(inH0Ax[tokenIndex][slot])
			if err != nil {
				return nil, err
			}
			h0y, err := utils.ParseBigInt(inH0Ay[tokenIndex][slot])
			if err != nil {
				return nil, err
			}
			note, err := utxo.NewUtxo(types.UtxoParams{
				Amount:            amount,
				Erc20TokenAddress: erc20TokenAddress,
				NullifyingKey:     nullifyingKey,
				SpendingPublicKey: ownSpendingPublicKey,
				H0:                &types.JubPoint{h0x, h0y},
				TimeStamp:         inTimeStamps[tokenIndex][slot],
			})
			if err != nil {
				return nil, err
			}
			commitment, err := note.GetCommitment()
			if err != nil {
				return nil, err
			}
			spentCommitments[commitment] = struct{}{}
			nullifier, err := note.GetNullifier()
			if err != nil {
				return nil, err
			}
			row[slot] = nullifier
		}
		inNullifiers[tokenIndex] = row
	}

	if inputCommitments != nil {
		mismatch := len(spentCommitments) != len(inputCommitments)
		if !mismatch {
			for _, c := range inputCommitments {
				if _, ok := spentCommitments[c]; !ok {
					mismatch = true
					break
				}
			}
		}
		if mismatch {
			return nil, errors.New("enclave: prepare-tx spent a different set of notes than the caller authorized - refusing to sign")
		}
	}
	return inNullifiers, nil
}

func verifyExpectedOutputCommitment(amount *big.Int, params types.UtxoParams) (string, error) {
	if amount.Sign() == 0 {
		return "0", nil
	}
	params.Amount = amount
	note, err := utxo.NewUtxo(params)
	if err != nil {
		return "", err
	}
	return note.GetCommitment()
}

// Recipient notes come entirely from the caller's own recipientAddress/recipientAmounts, never from
// the enclave's outAmounts/outNoteH0.
func verifyComputeOutCommitments(
	erc20Addresses []string,
	selfOutputAmounts []string,
	outAmounts, outNoteH0Ax, outNoteH0Ay [][]string,
	outTimeStamp string,
	nullifyingKey string,
	spendingPublicKey [2]string,
	recipientAddress []string,
	recipientAmounts [][]string,
) ([][]string, error) {
	if len(recipientAddress) != len(recipientAmounts) {
		return nil, errors.New("enclave: prepare-tx request has mismatched recipientAddress/recipientAmounts - refusing to sign")
	}
	selfSlotCount := 1
	if selfOutputAmounts != nil {
		selfSlotCount = len(selfOutputAmounts)
	}
	if len(outAmounts) != len(erc20Addresses) || len(outNoteH0Ax) != len(erc20Addresses) || len(outNoteH0Ay) != len(erc20Addresses) {
		return nil, errors.New("enclave: response out-note arrays do not match the number of tokens - refusing to sign")
	}
	ownSpendingPublicKey, err := stringsToBigInts(spendingPublicKey[:])
	if err != nil {
		return nil, err
	}

	result := make([][]string, len(erc20Addresses))
	for tokenIndex, erc20TokenAddress := range erc20Addresses {
		if len(outAmounts[tokenIndex]) < selfSlotCount || len(outNoteH0Ax[tokenIndex]) < selfSlotCount || len(outNoteH0Ay[tokenIndex]) < selfSlotCount {
			return nil, errors.New("enclave: response out-note row is shorter than the requested self-output slots - refusing to sign")
		}
		row := make([]string, 0, selfSlotCount+len(recipientAddress))
		for slot := 0; slot < selfSlotCount; slot++ {
			amount, err := utils.ParseBigInt(outAmounts[tokenIndex][slot])
			if err != nil {
				return nil, err
			}
			h0x, err := utils.ParseBigInt(outNoteH0Ax[tokenIndex][slot])
			if err != nil {
				return nil, err
			}
			h0y, err := utils.ParseBigInt(outNoteH0Ay[tokenIndex][slot])
			if err != nil {
				return nil, err
			}
			commitment, err := verifyExpectedOutputCommitment(amount, types.UtxoParams{
				Erc20TokenAddress: erc20TokenAddress,
				NullifyingKey:     nullifyingKey,
				SpendingPublicKey: ownSpendingPublicKey,
				H0:                &types.JubPoint{h0x, h0y},
				TimeStamp:         outTimeStamp,
			})
			if err != nil {
				return nil, err
			}
			row = append(row, commitment)
		}
		for recipientIndex, recipient := range recipientAddress {
			parts := strings.Split(recipient, ",")
			if len(parts) < 3 {
				return nil, fmt.Errorf("enclave: malformed recipientAddress entry %q", recipient)
			}
			h0x, err := utils.ParseBigInt(parts[1])
			if err != nil {
				return nil, err
			}
			h0y, err := utils.ParseBigInt(parts[2])
			if err != nil {
				return nil, err
			}
			amount, err := utils.ParseBigInt(recipientAmounts[recipientIndex][tokenIndex])
			if err != nil {
				return nil, err
			}
			commitment, err := verifyExpectedOutputCommitment(amount, types.UtxoParams{
				Erc20TokenAddress: erc20TokenAddress,
				StealthAddress:    parts[0],
				H0:                &types.JubPoint{h0x, h0y},
				TimeStamp:         outTimeStamp,
			})
			if err != nil {
				return nil, err
			}
			row = append(row, commitment)
		}
		result[tokenIndex] = row
	}
	return result, nil
}

func verifyAssertAmountsConserve(
	erc20Addresses []string,
	inAmounts, outAmounts [][]string,
	selfOutputAmounts []string,
	amountChanges []string,
) error {
	if len(inAmounts) != len(erc20Addresses) || len(outAmounts) != len(erc20Addresses) || len(amountChanges) != len(erc20Addresses) {
		return errors.New("enclave: response amount arrays do not match the number of tokens - refusing to sign")
	}
	selfSlotCount := 1
	if selfOutputAmounts != nil {
		selfSlotCount = len(selfOutputAmounts)
	}
	for tokenIndex := range erc20Addresses {
		if len(outAmounts[tokenIndex]) < selfSlotCount {
			return errors.New("enclave: response out-amount row is shorter than the requested self-output slots - refusing to sign")
		}
	}

	if len(selfOutputAmounts) > 0 && len(outAmounts) == 0 {
		return errors.New("enclave: response has no out-amount rows for the requested self-output slots - refusing to sign")
	}
	for slot, expected := range selfOutputAmounts {
		if outAmounts[0][slot] != expected {
			return errors.New("enclave: prepare-tx self-output amounts do not match the requested selfOutputAmounts - refusing to sign")
		}
	}

	for tokenIndex := range erc20Addresses {
		inputTotal := big.NewInt(0)
		for _, a := range inAmounts[tokenIndex] {
			v, err := utils.ParseBigInt(a)
			if err != nil {
				return err
			}
			inputTotal.Add(inputTotal, v)
		}
		selfOutputTotal := big.NewInt(0)
		for _, a := range outAmounts[tokenIndex][:selfSlotCount] {
			v, err := utils.ParseBigInt(a)
			if err != nil {
				return err
			}
			selfOutputTotal.Add(selfOutputTotal, v)
		}
		change, err := utils.ParseBigInt(amountChanges[tokenIndex])
		if err != nil {
			return err
		}
		if new(big.Int).Add(inputTotal, change).Cmp(selfOutputTotal) != 0 {
			return errors.New("enclave: prepare-tx self-output amount does not match input total + requested amountChanges - refusing to sign")
		}
	}
	return nil
}

func verifyComputeAmountChangesSigned(erc20Addresses []string, amountChanges []string, recipientAmounts [][]string) ([]*big.Int, error) {
	out := make([]*big.Int, len(erc20Addresses))
	for tokenIndex := range erc20Addresses {
		selfDelta, err := utils.ParseBigInt(amountChanges[tokenIndex])
		if err != nil {
			return nil, err
		}
		recipientTotal := big.NewInt(0)
		for _, perToken := range recipientAmounts {
			v, err := utils.ParseBigInt(perToken[tokenIndex])
			if err != nil {
				return nil, err
			}
			recipientTotal.Add(recipientTotal, v)
		}
		out[tokenIndex] = new(big.Int).Add(selfDelta, recipientTotal)
	}
	return out, nil
}

// Matters only when onChainCreation has a true entry: DepositOnChainUtxosExternalAction.sol uses
// this tx-level pair as the on-chain-created leaf's stealth address, so it must be caller-owned.
func verifyAssertTxLevelStealthPairOwnedByCaller(onChainCreation []bool, h0Ax, h0Ay, outH1Ax, outH1Ay, nullifyingKey string) error {
	if !anyTrue(onChainCreation) {
		return nil
	}
	h0x, err := utils.ParseBigInt(h0Ax)
	if err != nil {
		return err
	}
	h0y, err := utils.ParseBigInt(h0Ay)
	if err != nil {
		return err
	}
	h1x, err := utils.ParseBigInt(outH1Ax)
	if err != nil {
		return err
	}
	h1y, err := utils.ParseBigInt(outH1Ay)
	if err != nil {
		return err
	}
	ok, err := cryptokeys.VerifyStealthPair(types.JubPoint{h0x, h0y}, types.JubPoint{h1x, h1y}, nullifyingKey, false)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("enclave: prepare-tx tx-level stealth pair is not derived from the caller's own key - refusing to sign")
	}
	return nil
}

func verifyCreateCallDataHash(
	chainID, publicSignalCount int,
	relay, externalAddress string,
	externalActionIDHash *big.Int,
	externalActionMetadata string,
	emporiumMessage *big.Int,
	encryptedOutputsHex [][]string,
	onChainEncryptedOutputHex string,
	slippageValues []*big.Int,
	onChainCreation []bool,
	feeStructure types.FeeStructure,
	originalSender string,
	createBlockedUtxos bool,
) (*big.Int, error) {
	isTron := constants.IsTronLike(chainID)

	encryptedOutputs := make([][][]byte, len(encryptedOutputsHex))
	for i, inner := range encryptedOutputsHex {
		encryptedOutputs[i] = make([][]byte, len(inner))
		for j, v := range inner {
			encryptedOutputs[i][j] = common.FromHex(v)
		}
	}

	args1 := abi.Arguments{{Type: verifyAbiUint16}, {Type: verifyAbiAddress}, {Type: verifyAbiUint256}, {Type: verifyAbiExternalActionTuple}}
	values1 := []any{
		uint16(publicSignalCount),
		common.HexToAddress(relay),
		emporiumMessage,
		verifyExternalActionTuple{
			ExternalAddress:        common.HexToAddress(externalAddress),
			ExternalActionId:       externalActionIDHash,
			ExternalActionMetadata: common.FromHex(externalActionMetadata),
		},
	}
	if !isTron {
		args1 = append(args1, abi.Argument{Type: verifyAbiInt256Arr})
		values1 = append(values1, slippageValues)
	}
	encoded1, err := args1.Pack(values1...)
	if err != nil {
		return nil, fmt.Errorf("encode calldata hash part 1: %w", err)
	}

	hookData := verifyHookDataTuple{
		PreHookContract:  common.Address{},
		PostHookContract: common.Address{},
		PreHookMetadata:  common.FromHex("0x00"),
		PostHookMetadata: common.FromHex("0x00"),
	}
	feeStructureValue := verifyFeeStructureTuple{
		FeeToken:     common.HexToAddress(feeStructure.FeeToken),
		FlatFee:      feeStructure.FlatFee,
		VariableRate: feeStructure.VariableRate,
	}

	args2 := abi.Arguments{{Type: verifyAbiHookTuple}, {Type: verifyAbiBytesArr2}}
	values2 := []any{hookData, encryptedOutputs}
	if isTron {
		args2 = append(args2, abi.Argument{Type: verifyAbiFeeStructureTuple}, abi.Argument{Type: verifyAbiInt256Arr})
		values2 = append(values2, feeStructureValue, slippageValues)
		args2 = append(args2, abi.Argument{Type: verifyAbiAddress}, abi.Argument{Type: verifyAbiBytes}, abi.Argument{Type: verifyAbiBool})
		values2 = append(values2, common.HexToAddress(originalSender), common.FromHex("0x"), createBlockedUtxos)
	} else {
		args2 = append(args2, abi.Argument{Type: verifyAbiBytes}, abi.Argument{Type: verifyAbiFeeStructureTuple})
		values2 = append(values2, common.FromHex(onChainEncryptedOutputHex), feeStructureValue)
		args2 = append(args2, abi.Argument{Type: verifyAbiBoolArr}, abi.Argument{Type: verifyAbiAddress}, abi.Argument{Type: verifyAbiBytes})
		values2 = append(values2, onChainCreation, common.HexToAddress(originalSender), common.FromHex("0x"))
	}

	encoded2, err := args2.Pack(values2...)
	if err != nil {
		return nil, fmt.Errorf("encode calldata hash part 2: %w", err)
	}

	hash1 := new(big.Int).SetBytes(gethcrypto.Keccak256(encoded1))
	hash2 := new(big.Int).SetBytes(gethcrypto.Keccak256(encoded2))

	finalEncoded, err := (abi.Arguments{{Type: verifyAbiUint256}, {Type: verifyAbiUint256}}).Pack(hash1, hash2)
	if err != nil {
		return nil, fmt.Errorf("encode calldata hash final: %w", err)
	}

	calldataHash := new(big.Int).SetBytes(gethcrypto.Keccak256(finalEncoded))
	return calldataHash.Mod(calldataHash, crypto.FieldP), nil
}

func verifyPackBySolidityType(solidityTypes []string, values []any) ([]byte, error) {
	args := make(abi.Arguments, len(solidityTypes))
	for i, t := range solidityTypes {
		switch t {
		case "uint256":
			args[i] = abi.Argument{Type: verifyAbiUint256}
		case "uint256[]":
			args[i] = abi.Argument{Type: verifyAbiUint256Arr}
		case "address":
			args[i] = abi.Argument{Type: verifyAbiAddress}
		}
	}
	return args.Pack(values...)
}

func verifyPackAndHash(solidityTypes []string, values []any) (*big.Int, error) {
	encoded, err := verifyPackBySolidityType(solidityTypes, values)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(gethcrypto.Keccak256(encoded)), nil
}

// Split into two parts to dodge Solidity "stack too deep", same as signedMessageHash.ts.
var (
	verifyEvmSignedMessageHashTypes1 = []string{
		"uint256", "address", "uint256", "uint256[]", "uint256[]",
		"uint256", "uint256[]", "uint256[]", "uint256", "uint256",
	}
	verifyEvmSignedMessageHashTypes2 = []string{"uint256", "uint256", "uint256", "uint256"}
	verifyTronSignedMessageHashTypes = []string{
		"uint256", "uint256[]", "uint256[]",
		"uint256", "uint256[]", "uint256[]", "uint256", "uint256",
	}
)

func verifyTokenAddressesAsInts(erc20TokenAddresses []string) []*big.Int {
	out := make([]*big.Int, len(erc20TokenAddresses))
	for i, a := range erc20TokenAddresses {
		out[i] = new(big.Int).SetBytes(common.FromHex(a))
	}
	return out
}

func verifyComputeSignedMessageHashEvm(
	chainID *big.Int, verifyingContract string,
	rootHashHinkal *big.Int, erc20TokenAddresses []string, amountChanges []*big.Int,
	outTimeStamp *big.Int, inNullifiersFlat, outCommitmentsFlat []*big.Int,
	calldataHash, message, outH1Ax, outH1Ay, h0Ax, h0Ay *big.Int,
) (*big.Int, error) {
	hash1, err := verifyPackAndHash(verifyEvmSignedMessageHashTypes1, []any{
		chainID, common.HexToAddress(verifyingContract), rootHashHinkal, verifyTokenAddressesAsInts(erc20TokenAddresses),
		amountChanges, outTimeStamp, inNullifiersFlat, outCommitmentsFlat, calldataHash, message,
	})
	if err != nil {
		return nil, fmt.Errorf("encode signed message hash (part 1): %w", err)
	}
	hash2, err := verifyPackAndHash(verifyEvmSignedMessageHashTypes2, []any{outH1Ax, outH1Ay, h0Ax, h0Ay})
	if err != nil {
		return nil, fmt.Errorf("encode signed message hash (part 2): %w", err)
	}
	hash, err := verifyPackAndHash([]string{"uint256", "uint256"}, []any{hash1, hash2})
	if err != nil {
		return nil, fmt.Errorf("encode signed message hash (final): %w", err)
	}
	return hash.Mod(hash, crypto.FieldP), nil
}

func verifyComputeSignedMessageHashTron(
	rootHashHinkal *big.Int, erc20TokenAddresses []string, amountChanges []*big.Int,
	outTimeStamp *big.Int, inNullifiersFlat, outCommitmentsFlat []*big.Int,
	calldataHash, message *big.Int,
) (*big.Int, error) {
	hash, err := verifyPackAndHash(verifyTronSignedMessageHashTypes, []any{
		rootHashHinkal, verifyTokenAddressesAsInts(erc20TokenAddresses), amountChanges, outTimeStamp,
		inNullifiersFlat, outCommitmentsFlat, calldataHash, message,
	})
	if err != nil {
		return nil, fmt.Errorf("encode tron signed message hash: %w", err)
	}
	return hash.Mod(hash, crypto.FieldP), nil
}

// Includes the Tron branch that snarkjs.CalcPublicSignalCount omits (that one's EVM-only).
func verifyCalcPublicSignalCount(isTron bool, erc20Count, amountChangesCount, inNullifiersFlatLen, outCommitmentsFlatLen int) int {
	if erc20Count == 0 {
		return 3
	}
	base := 1 + // rootHashHinkal
		1 + // signedMessageHash
		erc20Count +
		amountChangesCount +
		1 + // outTimeStamp
		inNullifiersFlatLen +
		outCommitmentsFlatLen +
		1 + // calldataHash
		1 // emporiumMessage
	if isTron {
		return base + 1 + 1 // newRootHash, insertedLeafIndex
	}
	return base + 1 + 1 + 1 + 1 + 1 // outH0Ay, outH1Ax, outH1Ay, signs, outStealthAddress
}

// Pins GetSolanaCalldataHash's dimensions input to counts the caller can derive itself, instead of
// trusting the enclave's reported value.
func verifyComputeExpectedDimensions(tokenCount int, inNullifiers, outCommitments [][]string) types.DimDataType {
	nullifierAmount := 0
	if len(inNullifiers) > 0 {
		nullifierAmount = len(inNullifiers[0])
	}
	outputAmount := 0
	if len(outCommitments) > 0 {
		outputAmount = len(outCommitments[0])
	}
	return types.DimDataType{
		TokenNumber:     tokenCount,
		NullifierAmount: nullifierAmount,
		OutputAmount:    outputAmount,
	}
}

func verifyAssertDimensionsMatch(dimensions, expected types.DimDataType, route string) error {
	if dimensions.TokenNumber != expected.TokenNumber ||
		dimensions.NullifierAmount != expected.NullifierAmount ||
		dimensions.OutputAmount != expected.OutputAmount {
		return fmt.Errorf("enclave: %s dimensions do not match the caller's own data - refusing to sign", route)
	}
	return nil
}

func verifyComputeCalldataHashAndMessage(
	chainID int,
	payload types.PrepareTxRequestType,
	response types.PrepareTxResponseType,
	inNullifiers, outCommitments [][]string,
) (calldataHash, message *big.Int, amountChangesModWrapped []*big.Int, err error) {
	messageSeed, err := utils.ParseBigInt(response.MessageSeed)
	if err != nil {
		return
	}
	message, err = crypto.PoseidonBig(messageSeed)
	if err != nil {
		return
	}

	relay := payload.Relay
	if relay == "" {
		relay = constants.ZeroAddress
	}
	relayed := relay != constants.ZeroAddress

	feeStructure := types.ZeroFeeStructure()
	if relayed && payload.FeeStructure != nil {
		var flatFee, variableRate *big.Int
		flatFee, err = utils.ParseBigInt(payload.FeeStructure.FlatFee)
		if err != nil {
			return
		}
		variableRate, err = utils.ParseBigInt(payload.FeeStructure.VariableRate)
		if err != nil {
			return
		}
		feeStructure = types.FeeStructure{FeeToken: payload.FeeStructure.FeeToken, FlatFee: flatFee, VariableRate: variableRate}
	}

	externalActionMetadata := payload.ExternalActionMetadata
	if externalActionMetadata == "" {
		externalActionMetadata = "0x00"
	}
	onChainCreation := expandBoolPerToken(payload.OnChainCreation, len(payload.Erc20Addresses))

	var amountChangesSigned []*big.Int
	amountChangesSigned, err = verifyComputeAmountChangesSigned(payload.Erc20Addresses, payload.AmountChanges, payload.RecipientAmounts)
	if err != nil {
		return
	}

	var slippageValues []*big.Int
	if payload.SlippageValues != nil {
		slippageValues, err = stringsToBigInts(payload.SlippageValues)
		if err != nil {
			return
		}
	} else {
		slippageValues = verifyGetSlippageValues(amountChangesSigned)
	}

	var inNullifiersFlat, outCommitmentsFlat []*big.Int
	inNullifiersFlat, err = verifyFlatToBigInts(inNullifiers)
	if err != nil {
		return
	}
	outCommitmentsFlat, err = verifyFlatToBigInts(outCommitments)
	if err != nil {
		return
	}

	isTron := constants.IsTronLike(chainID)
	publicSignalCount := verifyCalcPublicSignalCount(isTron, len(payload.Erc20Addresses), len(amountChangesSigned), len(inNullifiersFlat), len(outCommitmentsFlat))

	externalActionIDHash := verifyGetExternalActionIDHash(payload.ExternalActionID)

	originalSender := payload.OriginalSender
	if originalSender == "" {
		originalSender = verifyGetOriginalSender(externalAddressOrZero(payload.ExternalAddress), relay)
	}

	calldataHash, err = verifyCreateCallDataHash(
		chainID, publicSignalCount, relay, payload.ExternalAddress,
		externalActionIDHash, externalActionMetadata, message,
		response.EncryptedOutputs, response.OnChainEncryptedOutput,
		slippageValues, onChainCreation, feeStructure,
		originalSender, payload.CreateBlockedUtxos,
	)
	if err != nil {
		return
	}

	amountChangesModWrapped = make([]*big.Int, len(amountChangesSigned))
	for i, a := range amountChangesSigned {
		amountChangesModWrapped[i] = verifyWrapAmountChange(a)
	}
	return
}

func verifyAssertSignedMessageHashMatches(
	chainID int,
	hinkalAddress string,
	response types.PrepareTxResponseType,
	inNullifiers, outCommitments [][]string,
	calldataHash, message *big.Int,
	amountChangesModWrapped []*big.Int,
) error {
	rootHashHinkal, err := utils.ParseBigInt(response.RootHashHinkal)
	if err != nil {
		return err
	}
	outTimeStamp, err := utils.ParseBigInt(response.OutTimeStamp)
	if err != nil {
		return err
	}
	inNullifiersFlat, err := verifyFlatToBigInts(inNullifiers)
	if err != nil {
		return err
	}
	outCommitmentsFlat, err := verifyFlatToBigInts(outCommitments)
	if err != nil {
		return err
	}

	var recomputed *big.Int
	if constants.IsTronLike(chainID) {
		recomputed, err = verifyComputeSignedMessageHashTron(rootHashHinkal, response.Request.Erc20Addresses, amountChangesModWrapped, outTimeStamp, inNullifiersFlat, outCommitmentsFlat, calldataHash, message)
	} else {
		var outH1Ax, outH1Ay, h0Ax, h0Ay *big.Int
		outH1Ax, err = utils.ParseBigInt(response.OutH1Ax)
		if err != nil {
			return err
		}
		outH1Ay, err = utils.ParseBigInt(response.OutH1Ay)
		if err != nil {
			return err
		}
		h0Ax, err = utils.ParseBigInt(response.H0Ax)
		if err != nil {
			return err
		}
		h0Ay, err = utils.ParseBigInt(response.H0Ay)
		if err != nil {
			return err
		}
		recomputed, err = verifyComputeSignedMessageHashEvm(
			big.NewInt(int64(chainID)), hinkalAddress, rootHashHinkal,
			response.Request.Erc20Addresses, amountChangesModWrapped, outTimeStamp,
			inNullifiersFlat, outCommitmentsFlat, calldataHash, message,
			outH1Ax, outH1Ay, h0Ax, h0Ay,
		)
	}
	if err != nil {
		return err
	}
	expected, err := utils.ParseBigInt(response.SignedMessageHash)
	if err != nil {
		return err
	}
	if recomputed.Cmp(expected) != 0 {
		return errors.New("enclave: prepare-tx signedMessageHash does not match one built from the caller's own data - refusing to sign")
	}
	return nil
}

func verifyPrepareTxResponse(chainID int, payload types.PrepareTxRequestType, response types.PrepareTxResponseType) error {
	var authorizedInputCommitments []string
	if payload.InputCommitments != nil {
		authorizedInputCommitments = payload.InputCommitments
	} else {
		specCommitments, err := verifyCollectSpeculativeInputCommitments(payload.Speculative)
		if err != nil {
			return err
		}
		if specCommitments != nil {
			authorizedInputCommitments = specCommitments
		} else if payload.ForceEmptyUtxos {
			authorizedInputCommitments = []string{}
		}
	}

	inNullifiers, err := verifyComputeInNullifiers(payload.Erc20Addresses, response.InAmounts, response.InH0Ax, response.InH0Ay, response.InTimeStamps, payload.NullifyingKey, payload.SpendingPublicKey, authorizedInputCommitments)
	if err != nil {
		return err
	}
	outCommitments, err := verifyComputeOutCommitments(payload.Erc20Addresses, payload.SelfOutputAmounts, response.OutAmounts, response.OutNoteH0Ax, response.OutNoteH0Ay, response.OutTimeStamp, payload.NullifyingKey, payload.SpendingPublicKey, payload.RecipientAddress, payload.RecipientAmounts)
	if err != nil {
		return err
	}
	if err := verifyAssertAmountsConserve(payload.Erc20Addresses, response.InAmounts, response.OutAmounts, payload.SelfOutputAmounts, payload.AmountChanges); err != nil {
		return err
	}
	if err := verifyAssertTxLevelStealthPairOwnedByCaller(payload.OnChainCreation, response.H0Ax, response.H0Ay, response.OutH1Ax, response.OutH1Ay, payload.NullifyingKey); err != nil {
		return err
	}
	calldataHash, message, amountChangesModWrapped, err := verifyComputeCalldataHashAndMessage(chainID, payload, response, inNullifiers, outCommitments)
	if err != nil {
		return err
	}
	return verifyAssertSignedMessageHashMatches(chainID, payload.HinkalAddress, response, inNullifiers, outCommitments, calldataHash, message, amountChangesModWrapped)
}

// amountChanges comes from the job's own reported input total, not the request - the request
// carries no amounts, since only the enclave knows which stuck notes exist.
func verifyStuckWithdrawJobSignedMessageHash(
	chainID int,
	hinkalAddress, erc20TokenAddress, relay, externalAddress string,
	feeStructure types.FeeStructure,
	nullifyingKey string,
	spendingPublicKey [2]string,
	job types.PreparedStuckWithdrawJobType,
) error {
	inNullifiers, err := verifyComputeInNullifiers([]string{erc20TokenAddress}, job.InAmounts, job.InH0Ax, job.InH0Ay, job.InTimeStamps, nullifyingKey, spendingPublicKey, nil)
	if err != nil {
		return err
	}
	outCommitments, err := verifyComputeOutCommitments([]string{erc20TokenAddress}, nil, job.OutAmounts, job.OutNoteH0Ax, job.OutNoteH0Ay, job.OutTimeStamp, nullifyingKey, spendingPublicKey, nil, nil)
	if err != nil {
		return err
	}

	inputTotal := big.NewInt(0)
	for _, a := range job.InAmounts[0] {
		v, err := utils.ParseBigInt(a)
		if err != nil {
			return err
		}
		inputTotal.Add(inputTotal, v)
	}
	amountChangesSigned := new(big.Int).Neg(inputTotal)
	if err := verifyAssertAmountsConserve([]string{erc20TokenAddress}, job.InAmounts, job.OutAmounts, nil, []string{amountChangesSigned.String()}); err != nil {
		return err
	}
	amountChangesModWrapped := []*big.Int{verifyWrapAmountChange(amountChangesSigned)}

	messageSeed, err := utils.ParseBigInt(job.MessageSeed)
	if err != nil {
		return err
	}
	message, err := crypto.PoseidonBig(messageSeed)
	if err != nil {
		return err
	}
	slippageValues := verifyGetSlippageValues([]*big.Int{amountChangesSigned})

	inNullifiersFlat, err := verifyFlatToBigInts(inNullifiers)
	if err != nil {
		return err
	}
	outCommitmentsFlat, err := verifyFlatToBigInts(outCommitments)
	if err != nil {
		return err
	}
	isTron := constants.IsTronLike(chainID)
	publicSignalCount := verifyCalcPublicSignalCount(isTron, 1, 1, len(inNullifiersFlat), len(outCommitmentsFlat))

	calldataHash, err := verifyCreateCallDataHash(
		chainID, publicSignalCount, relay, externalAddress,
		big.NewInt(0), "0x00", message,
		job.EncryptedOutputs, job.OnChainEncryptedOutput,
		slippageValues, []bool{false}, feeStructure,
		verifyGetOriginalSender(externalAddressOrZero(externalAddress), relay),
		false,
	)
	if err != nil {
		return err
	}

	rootHashHinkal, err := utils.ParseBigInt(job.RootHashHinkal)
	if err != nil {
		return err
	}
	outTimeStamp, err := utils.ParseBigInt(job.OutTimeStamp)
	if err != nil {
		return err
	}

	var recomputed *big.Int
	if isTron {
		recomputed, err = verifyComputeSignedMessageHashTron(rootHashHinkal, []string{erc20TokenAddress}, amountChangesModWrapped, outTimeStamp, inNullifiersFlat, outCommitmentsFlat, calldataHash, message)
	} else {
		var outH1Ax, outH1Ay, h0Ax, h0Ay *big.Int
		outH1Ax, err = utils.ParseBigInt(job.OutH1Ax)
		if err != nil {
			return err
		}
		outH1Ay, err = utils.ParseBigInt(job.OutH1Ay)
		if err != nil {
			return err
		}
		h0Ax, err = utils.ParseBigInt(job.H0Ax)
		if err != nil {
			return err
		}
		h0Ay, err = utils.ParseBigInt(job.H0Ay)
		if err != nil {
			return err
		}
		recomputed, err = verifyComputeSignedMessageHashEvm(big.NewInt(int64(chainID)), hinkalAddress, rootHashHinkal, []string{erc20TokenAddress}, amountChangesModWrapped, outTimeStamp, inNullifiersFlat, outCommitmentsFlat, calldataHash, message, outH1Ax, outH1Ay, h0Ax, h0Ay)
	}
	if err != nil {
		return err
	}
	expected, err := utils.ParseBigInt(job.SignedMessageHash)
	if err != nil {
		return err
	}
	if recomputed.Cmp(expected) != 0 {
		return errors.New("enclave: prepare-stuck-withdraw job signedMessageHash does not match one built from the caller's own data - refusing to sign")
	}
	return nil
}

func intsToBytes(values []int) []byte {
	out := make([]byte, len(values))
	for i, v := range values {
		out[i] = byte(v)
	}
	return out
}

// crypto_box_seal ciphertext can't be recomputed, but zeroing it the same way as the enclave keeps
// it bound into the hash.
func verifyComputeSolanaCalldataHashAndMessage(
	payload types.PrepareSolanaTxRequestType,
	response types.PrepareSolanaTxResponseType,
	inNullifiers, outCommitments [][]string,
) (calldataHash, message *big.Int, err error) {
	messageSeed, err := utils.ParseBigInt(response.MessageSeed)
	if err != nil {
		return
	}
	message, err = crypto.PoseidonBig(messageSeed)
	if err != nil {
		return
	}

	if dimErr := verifyAssertDimensionsMatch(
		response.Dimensions,
		verifyComputeExpectedDimensions(len(payload.MintAddresses), inNullifiers, outCommitments),
		"prepare-solana-tx",
	); dimErr != nil {
		err = dimErr
		return
	}

	onChainCreation := expandBoolPerToken(payload.OnChainCreation, len(payload.MintAddresses))
	hasOnChainCreation := anyTrue(onChainCreation)

	encryptedOutputsForHash := make([][]byte, 0)
	for tokenIndex, tokenOutputs := range response.EncryptedOutputs {
		zeroed := tokenIndex < len(onChainCreation) && onChainCreation[tokenIndex]
		for _, hexStr := range tokenOutputs {
			if zeroed {
				encryptedOutputsForHash = append(encryptedOutputsForHash, []byte{})
			} else {
				encryptedOutputsForHash = append(encryptedOutputsForHash, common.FromHex(hexStr))
			}
		}
	}
	onChainEncryptedOutputHex := "0x"
	if hasOnChainCreation {
		onChainEncryptedOutputHex = response.OnChainEncryptedOutput
	}

	instructions := make([]solanautils.HinkalInstruction, len(payload.HinkalInstructions))
	for i, ins := range payload.HinkalInstructions {
		instructions[i] = solanautils.HinkalInstruction{
			AccountIndexes: intsToBytes(ins.AccountIndexes),
			Data:           intsToBytes(ins.Data),
			ProgramIndex:   ins.ProgramIndex,
		}
	}
	remainingAccounts := make([]solana.AccountMeta, len(payload.RemainingAccounts))
	for i, acc := range payload.RemainingAccounts {
		var pk solana.PublicKey
		pk, err = solana.PublicKeyFromBase58(acc.Pubkey)
		if err != nil {
			return
		}
		remainingAccounts[i] = solana.AccountMeta{PublicKey: pk, IsWritable: acc.IsWritable, IsSigner: acc.IsSigner}
	}

	recipientPk, err := solana.PublicKeyFromBase58(payload.Recipient)
	if err != nil {
		return
	}
	signer := payload.Signer
	if signer == "" {
		signer = payload.RelayAddress
	}
	signerPk, err := solana.PublicKeyFromBase58(signer)
	if err != nil {
		return
	}

	relayerFee := big.NewInt(0)
	if payload.RelayerFee != "" {
		relayerFee, err = utils.ParseBigInt(payload.RelayerFee)
		if err != nil {
			return
		}
	}
	variableRate := big.NewInt(0)
	if payload.VariableRate != "" {
		variableRate, err = utils.ParseBigInt(payload.VariableRate)
		if err != nil {
			return
		}
	}

	calldataHash = solanautils.GetSolanaCalldataHash(
		response.Dimensions, recipientPk, signerPk,
		encryptedOutputsForHash, common.FromHex(onChainEncryptedOutputHex),
		relayerFee, variableRate, instructions, remainingAccounts,
	)
	return
}

func verifyComputeSolanaAmountChangesSigned(payload types.PrepareSolanaTxRequestType) ([]*big.Int, error) {
	out := make([]*big.Int, len(payload.MintAddresses))
	for tokenIndex := range payload.MintAddresses {
		selfDelta, err := utils.ParseBigInt(payload.AmountChanges[tokenIndex])
		if err != nil {
			return nil, err
		}
		recipientAmount := big.NewInt(0)
		if payload.RecipientAmounts != nil {
			recipientAmount, err = utils.ParseBigInt(payload.RecipientAmounts[tokenIndex])
			if err != nil {
				return nil, err
			}
		}
		out[tokenIndex] = new(big.Int).Add(selfDelta, recipientAmount)
	}
	return out, nil
}

func verifyAssertSolanaSignedMessageHashMatches(
	response types.PrepareSolanaTxResponseType,
	mintAccountPart1, mintAccountPart2 []*big.Int,
	inNullifiers, outCommitments [][]string,
	calldataHash, message *big.Int,
	amountChangesModWrapped []*big.Int,
	swapperAccountAdditionalSeed *big.Int,
) error {
	rootHashHinkal, err := utils.ParseBigInt(response.RootHashHinkal)
	if err != nil {
		return err
	}
	outTimeStamp, err := utils.ParseBigInt(response.OutTimeStamp)
	if err != nil {
		return err
	}
	outH1Ay, err := utils.ParseBigInt(response.OutH1Ay)
	if err != nil {
		return err
	}
	h0Ax, err := utils.ParseBigInt(response.H0Ax)
	if err != nil {
		return err
	}
	h0Ay, err := utils.ParseBigInt(response.H0Ay)
	if err != nil {
		return err
	}

	recomputed, err := verifyComputeSignedMessageHashSolana(verifySolanaSignedMessageHashParams{
		RootHashHinkal:               rootHashHinkal,
		MintAccountPart1:             mintAccountPart1,
		MintAccountPart2:             mintAccountPart2,
		AmountChanges:                amountChangesModWrapped,
		OutTimeStamp:                 outTimeStamp,
		InNullifiers:                 inNullifiers,
		OutCommitments:               outCommitments,
		CalldataHash:                 calldataHash,
		Message:                      message,
		SwapperAccountAdditionalSeed: swapperAccountAdditionalSeed,
		OutH1Ay:                      outH1Ay,
		H0Ax:                         h0Ax,
		H0Ay:                         h0Ay,
	})
	if err != nil {
		return err
	}
	expected, err := utils.ParseBigInt(response.SignedMessageHash)
	if err != nil {
		return err
	}
	if recomputed.Cmp(expected) != 0 {
		return errors.New("enclave: prepare-solana-tx signedMessageHash does not match one built from the caller's own data - refusing to sign")
	}
	return nil
}

func verifyPrepareSolanaTxResponse(payload types.PrepareSolanaTxRequestType, response types.PrepareSolanaTxResponseType) error {
	compressedTokenAddresses := make([]string, len(payload.MintAddresses))
	mintParts := make([]solanautils.FormattedMintAddress, len(payload.MintAddresses))
	for i, mint := range payload.MintAddresses {
		formatted, err := solanautils.FormatMintAddress(mint)
		if err != nil {
			return err
		}
		compressedTokenAddresses[i] = formatted.CompressedAddress
		mintParts[i] = formatted
	}

	var solanaRecipientAddress []string
	if payload.RecipientAddress != "" {
		solanaRecipientAddress = []string{payload.RecipientAddress}
	}
	var solanaRecipientAmounts [][]string
	if payload.RecipientAmounts != nil {
		solanaRecipientAmounts = [][]string{payload.RecipientAmounts}
	}

	var authorizedInputCommitments []string
	if payload.InputCommitments != nil {
		authorizedInputCommitments = payload.InputCommitments
	} else {
		specCommitments, err := verifyCollectSpeculativeInputCommitments(payload.Speculative)
		if err != nil {
			return err
		}
		authorizedInputCommitments = specCommitments
	}

	inNullifiers, err := verifyComputeInNullifiers(compressedTokenAddresses, response.InAmounts, response.InH0Ax, response.InH0Ay, response.InTimeStamps, payload.NullifyingKey, payload.SpendingPublicKey, authorizedInputCommitments)
	if err != nil {
		return err
	}
	outCommitments, err := verifyComputeOutCommitments(compressedTokenAddresses, nil, response.OutAmounts, response.OutNoteH0Ax, response.OutNoteH0Ay, response.OutTimeStamp, payload.NullifyingKey, payload.SpendingPublicKey, solanaRecipientAddress, solanaRecipientAmounts)
	if err != nil {
		return err
	}
	if err := verifyAssertAmountsConserve(compressedTokenAddresses, response.InAmounts, response.OutAmounts, nil, payload.AmountChanges); err != nil {
		return err
	}

	calldataHash, message, err := verifyComputeSolanaCalldataHashAndMessage(payload, response, inNullifiers, outCommitments)
	if err != nil {
		return err
	}
	amountChangesSigned, err := verifyComputeSolanaAmountChangesSigned(payload)
	if err != nil {
		return err
	}
	amountChangesModWrapped := make([]*big.Int, len(amountChangesSigned))
	for i, a := range amountChangesSigned {
		amountChangesModWrapped[i] = verifyWrapAmountChange(a)
	}

	mintAccountPart1 := make([]*big.Int, len(mintParts))
	mintAccountPart2 := make([]*big.Int, len(mintParts))
	for i, p := range mintParts {
		mintAccountPart1[i] = p.MintAccountPart1
		mintAccountPart2[i] = p.MintAccountPart2
	}

	swapperAccountSalt := big.NewInt(0)
	if payload.SwapperAccountSalt != "" {
		swapperAccountSalt, err = utils.ParseBigInt(payload.SwapperAccountSalt)
		if err != nil {
			return err
		}
	}
	swapperAccountAdditionalSeed, err := crypto.PoseidonBig(swapperAccountSalt)
	if err != nil {
		return err
	}

	return verifyAssertSolanaSignedMessageHashMatches(response, mintAccountPart1, mintAccountPart2, inNullifiers, outCommitments, calldataHash, message, amountChangesModWrapped, swapperAccountAdditionalSeed)
}

// On-chain-creation never applies to a stuck withdraw.
func verifySolanaStuckWithdrawJobSignedMessageHash(
	mintAddress, signer, recipient string,
	feeStructure types.FeeStructure,
	nullifyingKey string,
	spendingPublicKey [2]string,
	job types.PreparedStuckWithdrawJobType,
) error {
	formatted, err := solanautils.FormatMintAddress(mintAddress)
	if err != nil {
		return err
	}
	compressedAddress := formatted.CompressedAddress

	inNullifiers, err := verifyComputeInNullifiers([]string{compressedAddress}, job.InAmounts, job.InH0Ax, job.InH0Ay, job.InTimeStamps, nullifyingKey, spendingPublicKey, nil)
	if err != nil {
		return err
	}
	outCommitments, err := verifyComputeOutCommitments([]string{compressedAddress}, nil, job.OutAmounts, job.OutNoteH0Ax, job.OutNoteH0Ay, job.OutTimeStamp, nullifyingKey, spendingPublicKey, nil, nil)
	if err != nil {
		return err
	}

	inputTotal := big.NewInt(0)
	for _, a := range job.InAmounts[0] {
		v, err := utils.ParseBigInt(a)
		if err != nil {
			return err
		}
		inputTotal.Add(inputTotal, v)
	}
	amountChangesSigned := new(big.Int).Neg(inputTotal)
	if err := verifyAssertAmountsConserve([]string{compressedAddress}, job.InAmounts, job.OutAmounts, nil, []string{amountChangesSigned.String()}); err != nil {
		return err
	}
	amountChangesModWrapped := verifyWrapAmountChange(amountChangesSigned)

	messageSeed, err := utils.ParseBigInt(job.MessageSeed)
	if err != nil {
		return err
	}
	message, err := crypto.PoseidonBig(messageSeed)
	if err != nil {
		return err
	}

	encryptedOutputsForHash := make([][]byte, 0)
	for _, tokenOutputs := range job.EncryptedOutputs {
		for _, hexStr := range tokenOutputs {
			encryptedOutputsForHash = append(encryptedOutputsForHash, common.FromHex(hexStr))
		}
	}

	recipientPk, err := solana.PublicKeyFromBase58(recipient)
	if err != nil {
		return err
	}
	signerPk, err := solana.PublicKeyFromBase58(signer)
	if err != nil {
		return err
	}

	if err := verifyAssertDimensionsMatch(
		job.Dimensions,
		verifyComputeExpectedDimensions(1, inNullifiers, outCommitments),
		"prepare-solana-stuck-withdraw",
	); err != nil {
		return err
	}

	calldataHash := solanautils.GetSolanaCalldataHash(
		job.Dimensions, recipientPk, signerPk,
		encryptedOutputsForHash, []byte{},
		feeStructure.FlatFee, feeStructure.VariableRate,
		[]solanautils.HinkalInstruction{}, []solana.AccountMeta{},
	)

	rootHashHinkal, err := utils.ParseBigInt(job.RootHashHinkal)
	if err != nil {
		return err
	}
	outTimeStamp, err := utils.ParseBigInt(job.OutTimeStamp)
	if err != nil {
		return err
	}
	outH1Ay, err := utils.ParseBigInt(job.OutH1Ay)
	if err != nil {
		return err
	}
	h0Ax, err := utils.ParseBigInt(job.H0Ax)
	if err != nil {
		return err
	}
	h0Ay, err := utils.ParseBigInt(job.H0Ay)
	if err != nil {
		return err
	}
	zeroSeed, err := crypto.PoseidonBig(big.NewInt(0))
	if err != nil {
		return err
	}

	recomputed, err := verifyComputeSignedMessageHashSolana(verifySolanaSignedMessageHashParams{
		RootHashHinkal:               rootHashHinkal,
		MintAccountPart1:             []*big.Int{formatted.MintAccountPart1},
		MintAccountPart2:             []*big.Int{formatted.MintAccountPart2},
		AmountChanges:                []*big.Int{amountChangesModWrapped},
		OutTimeStamp:                 outTimeStamp,
		InNullifiers:                 inNullifiers,
		OutCommitments:               outCommitments,
		CalldataHash:                 calldataHash,
		Message:                      message,
		SwapperAccountAdditionalSeed: zeroSeed,
		OutH1Ay:                      outH1Ay,
		H0Ax:                         h0Ax,
		H0Ay:                         h0Ay,
	})
	if err != nil {
		return err
	}
	expected, err := utils.ParseBigInt(job.SignedMessageHash)
	if err != nil {
		return err
	}
	if recomputed.Cmp(expected) != 0 {
		return errors.New("enclave: prepare-solana-stuck-withdraw job signedMessageHash does not match one built from the caller's own data - refusing to sign")
	}
	return nil
}
