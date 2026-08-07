package transactions

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errProoflessNotImplemented               = errors.New("transactions: prooflessDeposit is not implemented for this chain")
	errTokenAmountLengthMismatch             = errors.New("erc20Tokens and amountChanges length mismatch")
	errStealthLengthMismatch                 = errors.New("transactions: stealth address structures length mismatch")
	errDuplicateStealth                      = errors.New("transactions: duplicate randomization and stealth address pair detected in stealthAddressStructures")
	errRecipientEncryptionKeysRequired       = errors.New("transactions: recipientEncryptionKeysOverride is required whenever stealthAddressStructuresOverride is provided")
	errRecipientEncryptionKeysLengthMismatch = errors.New("transactions: recipientEncryptionKeysOverride length must be equal to erc20 tokens length")
)

type tokenWithBalance struct {
	token   types.ERC20Token
	balance *big.Int
}

func positiveAmount(amount *big.Int) *big.Int {
	if amount != nil && amount.Sign() > 0 {
		return new(big.Int).Set(amount)
	}
	return big.NewInt(0)
}

func aggregateAmountsForApproval(erc20Tokens []types.ERC20Token, amounts []*big.Int) []tokenWithBalance {
	out := make([]tokenWithBalance, 0, len(erc20Tokens))
	for i, token := range erc20Tokens {
		found := false
		for j := range out {
			if out[j].token.ChainID == token.ChainID && strings.EqualFold(out[j].token.Erc20TokenAddress, token.Erc20TokenAddress) {
				out[j].balance = new(big.Int).Add(out[j].balance, amounts[i])
				found = true
				break
			}
		}
		if !found {
			out = append(out, tokenWithBalance{token: token, balance: new(big.Int).Set(amounts[i])})
		}
	}
	return out
}

func assertNoDuplicateStealthAddressStructures(structures []types.StealthAddressStructure) error {
	seen := make(map[string]struct{}, len(structures))
	for _, s := range structures {
		key := fmt.Sprintf("%s:%s", s.H0x.String(), s.StealthAddress.String())
		if _, ok := seen[key]; ok {
			return errDuplicateStealth
		}
		seen[key] = struct{}{}
	}
	return nil
}

func getProoflessStealthAddressStructures(
	hinkal ihinkal.HinkalInternal,
	count int,
	overrides []types.StealthAddressStructure,
) ([]types.StealthAddressStructure, error) {
	if overrides != nil {
		if len(overrides) != count {
			return nil, errStealthLengthMismatch
		}
		return overrides, nil
	}

	structures := make([]types.StealthAddressStructure, count)
	for i := range structures {
		structure, err := pretransaction.GetStealthAddressStructureFromUserKeys(hinkal.GetUserKeys())
		if err != nil {
			return nil, err
		}
		structures[i] = structure
	}
	return structures, nil
}

func getOwnRecipientEncryptionKeys(hinkal ihinkal.HinkalInternal, count int) ([]string, error) {
	nullifyingKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	encPair, err := cryptokeys.GetEncryptionKeyPair(nullifyingKey)
	if err != nil {
		return nil, err
	}
	keys := make([]string, count)
	for i := range keys {
		keys[i] = encPair.PublicKey
	}
	return keys, nil
}

func buildProoflessEncryptedOutputs(structures []types.StealthAddressStructure, recipientEncryptionKeys []string) ([]string, error) {
	if len(recipientEncryptionKeys) != len(structures) {
		return nil, errRecipientEncryptionKeysLengthMismatch
	}
	outputs := make([]string, len(structures))
	for i, s := range structures {
		encryptionKey := recipientEncryptionKeys[i]
		if encryptionKey == "" {
			return nil, fmt.Errorf("missing encryptionKey for proofless deposit output %d: required so the enclave can attribute this UTXO to its owner", i)
		}
		encrypted, err := utxo.EncryptEncryptionKeyAndStealthAddress(encryptionKey, s.StealthAddress)
		if err != nil {
			return nil, err
		}
		outputs[i] = "0x" + hex.EncodeToString(encrypted)
	}
	return outputs, nil
}

func HinkalProoflessDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	stealthAddressStructuresOverride []types.StealthAddressStructure,
	recipientEncryptionKeysOverride []string,
	createBlockedUtxos bool,
	feeStructure types.ProoflessFeeStructure,
	orderID string,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) || constants.IsTronLike(chainID) {
		return types.TransactionRequest{}, "", errProoflessNotImplemented
	}

	if len(erc20Tokens) != len(amountChanges) {
		return types.TransactionRequest{}, "", errTokenAmountLengthMismatch
	}

	stealthAddressStructures, err := getProoflessStealthAddressStructures(hinkal, len(erc20Tokens), stealthAddressStructuresOverride)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if err := assertNoDuplicateStealthAddressStructures(stealthAddressStructures); err != nil {
		return types.TransactionRequest{}, "", err
	}

	if stealthAddressStructuresOverride != nil && recipientEncryptionKeysOverride == nil {
		return types.TransactionRequest{}, "", errRecipientEncryptionKeysRequired
	}
	if recipientEncryptionKeysOverride != nil && len(recipientEncryptionKeysOverride) != len(erc20Tokens) {
		return types.TransactionRequest{}, "", errRecipientEncryptionKeysLengthMismatch
	}
	recipientEncryptionKeys := recipientEncryptionKeysOverride
	if recipientEncryptionKeys == nil {
		recipientEncryptionKeys, err = getOwnRecipientEncryptionKeys(hinkal, len(erc20Tokens))
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
	}

	encryptedOutputs, err := buildProoflessEncryptedOutputs(stealthAddressStructures, recipientEncryptionKeys)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	hinkalAddr, err := contractabi.ProoflessDepositTargetAddress(chainID, feeStructure)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}

	data, err := contractabi.PackProoflessDeposit(
		chainID, erc20Addresses, amountChanges, stealthAddressStructures, encryptedOutputs,
		createBlockedUtxos, feeStructure, orderID,
	)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	isFeeNative := strings.EqualFold(feeStructure.FeeToken, constants.ZeroAddress)
	feeAmount := positiveAmount(feeStructure.FeeAmount)

	ethAmount := big.NewInt(0)
	for i, token := range erc20Tokens {
		if strings.EqualFold(token.Erc20TokenAddress, constants.ZeroAddress) {
			ethAmount.Add(ethAmount, amountChanges[i])
		}
	}
	if isFeeNative {
		ethAmount.Add(ethAmount, feeAmount)
	}
	var value *big.Int
	if ethAmount.Sign() > 0 {
		value = ethAmount
	}

	txReq := types.TransactionRequest{To: hinkalAddr, Data: data, Value: value}
	if returnTxData {
		return txReq, "", nil
	}

	adapter, err := hinkal.GetProviderAdapter(&chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	feeInclusiveTokens := erc20Tokens
	feeInclusiveAmounts := amountChanges
	if !isFeeNative && feeAmount.Sign() > 0 {
		feeInclusiveTokens = append(append([]types.ERC20Token{}, erc20Tokens...), types.ERC20Token{ChainID: chainID, Erc20TokenAddress: feeStructure.FeeToken})
		feeInclusiveAmounts = append(append([]*big.Int{}, amountChanges...), feeAmount)
	}

	tokensWithBalances := aggregateAmountsForApproval(feeInclusiveTokens, feeInclusiveAmounts)
	approvalTokens := make([]types.ERC20Token, 0, len(tokensWithBalances))
	approvalAmounts := make([]*big.Int, 0, len(tokensWithBalances))
	requirements := make([]web3.ApprovalRequirement, 0, len(tokensWithBalances))
	for _, twb := range tokensWithBalances {
		if strings.EqualFold(twb.token.Erc20TokenAddress, constants.ZeroAddress) || twb.balance.Sign() <= 0 {
			continue
		}
		approvalTokens = append(approvalTokens, twb.token)
		approvalAmounts = append(approvalAmounts, twb.balance)
		requirements = append(requirements, web3.ApprovalRequirement{TokenAddress: twb.token.Erc20TokenAddress, RequiredAmount: twb.balance})
	}

	if len(approvalTokens) > 0 {
		if err := web3.ApproveTokensToHinkal(ctx, adapter, chainID, common.HexToAddress(hinkalAddr), approvalTokens, approvalAmounts); err != nil {
			return types.TransactionRequest{}, "", err
		}
		ownerAddr, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		if err := web3.WaitForErc20Approvals(ctx, adapter, chainID, common.HexToAddress(ownerAddr), common.HexToAddress(hinkalAddr), requirements, 30, time.Second); err != nil {
			return types.TransactionRequest{}, "", err
		}
	}

	txResp, err := adapter.SendTransaction(ctx, txReq)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	emitDepositAdminData(ctx, hinkal, types.AdminShield, chainID, erc20Tokens, amountChanges)
	return txReq, txResp.Hash, nil
}
