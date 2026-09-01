package enclave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errPrepareSolanaTxEchoMismatch = errors.New("enclave: prepare-solana-tx response does not match the request sent - refusing to sign")

var errPrepareSolanaStuckWithdrawEchoMismatch = errors.New("enclave: prepare-solana-stuck-withdraw response does not match the request sent - refusing to sign")

func buildPrepareSolanaTxRequest(
	p types.PrepareSolanaTxParams,
	nullifyingKey string,
	spendingPublicKey [2]string,
) types.PrepareSolanaTxRequestType {
	req := types.PrepareSolanaTxRequestType{
		ChainID:            strconv.Itoa(p.ChainID),
		MintAddresses:      p.MintAddresses,
		AmountChanges:      bigIntStrings(p.AmountChanges),
		RelayAddress:       p.RelayAddress,
		Recipient:          p.Recipient,
		Signer:             p.Signer,
		FunctionName:       p.FunctionName,
		Accounts:           p.Accounts,
		OnChainCreation:    p.OnChainCreation,
		HinkalInstructions: p.HinkalInstructions,
		RemainingAccounts:  p.RemainingAccounts,
		RecipientAddress:   p.RecipientAddress,
		RecipientAmounts:   bigIntStrings(p.RecipientAmounts),
		InputCommitments:   p.InputCommitments,
		UseBlockedUtxos:    p.UseBlockedUtxos,
		RecipientAmount:    p.RecipientAmount,
		DisplayRecipient:   p.DisplayRecipient,
		Speculative:        p.Speculative,
		NullifyingKey:      nullifyingKey,
		SpendingPublicKey:  spendingPublicKey,
	}
	if p.RelayerFee != nil {
		req.RelayerFee = p.RelayerFee.String()
	}
	if p.VariableRate != nil {
		req.VariableRate = p.VariableRate.String()
	}
	if p.SwapperAccountSalt != nil {
		req.SwapperAccountSalt = p.SwapperAccountSalt.String()
	}
	if p.MessageSeed != nil {
		req.MessageSeed = p.MessageSeed.String()
	}
	return req
}

func jsonEqual(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == 0 && len(b) == 0
	}
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

func assertPrepareSolanaTxEchoMatches(payload types.PrepareSolanaTxRequestType, echoed types.EchoedPrepareSolanaTxRequestType) error {
	signer := payload.Signer
	if signer == "" {
		signer = payload.RelayAddress
	}
	functionName := payload.FunctionName
	if functionName == "" {
		functionName = "transact"
	}
	expected := types.EchoedPrepareSolanaTxRequestType{
		ChainID:            payload.ChainID,
		MintAddresses:      payload.MintAddresses,
		AmountChanges:      payload.AmountChanges,
		RelayAddress:       payload.RelayAddress,
		Recipient:          payload.Recipient,
		Signer:             signer,
		FunctionName:       functionName,
		OnChainCreation:    payload.OnChainCreation,
		RelayerFee:         defaultZeroAmount(payload.RelayerFee),
		VariableRate:       defaultZeroAmount(payload.VariableRate),
		HinkalInstructions: payload.HinkalInstructions,
		RemainingAccounts:  payload.RemainingAccounts,
		SwapperAccountSalt: defaultZeroAmount(payload.SwapperAccountSalt),
		RecipientAddress:   payload.RecipientAddress,
		RecipientAmounts:   payload.RecipientAmounts,
		InputCommitments:   payload.InputCommitments,
		UseBlockedUtxos:    payload.UseBlockedUtxos,
		MessageSeed:        payload.MessageSeed,
		RecipientAmount:    payload.RecipientAmount,
		DisplayRecipient:   payload.DisplayRecipient,
		Speculative:        payload.Speculative,
	}

	echoedAccounts := echoed.Accounts
	echoed.Accounts = nil
	if !reflect.DeepEqual(expected, echoed) {
		return errPrepareSolanaTxEchoMismatch
	}
	if !jsonEqual(payload.Accounts, echoedAccounts) {
		return errPrepareSolanaTxEchoMismatch
	}
	return nil
}

func defaultZeroAmount(value string) string {
	if value == "" {
		return "0"
	}
	return value
}

// Part 1 of the Solana enclave flow: the enclave picks the notes and builds the witness, returning
// signedMessageHash to sign locally plus the jobId Part 2 finishes.
func PrepareSolanaTxEnclaveCall(
	ctx context.Context,
	uk *cryptokeys.UserKeys,
	params types.PrepareSolanaTxParams,
) (types.PrepareSolanaTxResponseType, error) {
	nullifyingKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return types.PrepareSolanaTxResponseType{}, err
	}
	pair, err := uk.GetSpendingKeyPair()
	if err != nil {
		return types.PrepareSolanaTxResponseType{}, err
	}
	spendingPublicKey := [2]string{pair.PubSpendingBJJPoint[0].String(), pair.PubSpendingBJJPoint[1].String()}
	payload := buildPrepareSolanaTxRequest(params, nullifyingKey, spendingPublicKey)

	resp, err := enclavePostSealedSigned[types.PrepareSolanaTxResponseType](ctx, constants.EnclaveConfig.PrepareSolanaTx, payload)
	if err != nil {
		return types.PrepareSolanaTxResponseType{}, err
	}
	if err := assertPrepareSolanaTxEchoMatches(payload, resp.Request); err != nil {
		return types.PrepareSolanaTxResponseType{}, err
	}
	if err := verifyPrepareSolanaTxResponse(payload, resp); err != nil {
		return types.PrepareSolanaTxResponseType{}, err
	}
	return resp, nil
}

// Part 2: finishes the witness with the signature, runs proof gen and hands the args back.
func FinalizeSolanaTxEnclaveCall(ctx context.Context, jobID string, sig types.EddsaSignature) (types.FinalizeSolanaTxResponseType, error) {
	payload := types.FinalizeTxRequestType{
		JobID:          jobID,
		EddsaSignature: eddsaSignatureWire(sig),
	}
	return enclavePost[types.FinalizeSolanaTxResponseType](ctx, constants.EnclaveConfig.FinalizeSolanaTx, payload)
}

// Relayed counterpart to FinalizeSolanaTxEnclaveCall: henclave posts straight to the relayer.
func FinalizeSolanaTxEnclaveCallRelay(
	ctx context.Context,
	jobID string,
	sig types.EddsaSignature,
	chainID int,
	adminData *types.AdminDataType,
) (string, error) {
	payload := types.FinalizeSolanaTxRelayRequestType{
		JobID:          jobID,
		EddsaSignature: eddsaSignatureWire(sig),
		ChainID:        strconv.Itoa(chainID),
		AdminData:      adminData,
	}
	resp, err := enclavePost[types.FinalizeTxRelayResponseType](ctx, constants.EnclaveConfig.FinalizeSolanaTxRelay, payload)
	if err != nil {
		return "", err
	}
	return resp.TxHash, nil
}

func assertPrepareSolanaStuckWithdrawEchoMatches(
	payload types.PrepareSolanaStuckWithdrawRequestType,
	echoed types.EchoedPrepareSolanaStuckWithdrawRequestType,
) error {
	expected := types.EchoedPrepareSolanaStuckWithdrawRequestType{
		ChainID:               payload.ChainID,
		MintAddress:           payload.MintAddress,
		Recipient:             payload.Recipient,
		RelayAddress:          payload.RelayAddress,
		Signer:                payload.RelayAddress,
		FeeStructures:         payload.FeeStructures,
		HashedEthereumAddress: payload.HashedEthereumAddress,
	}

	echoedAccounts := echoed.Accounts
	echoed.Accounts = nil
	if !reflect.DeepEqual(expected, echoed) {
		return errPrepareSolanaStuckWithdrawEchoMismatch
	}
	if !jsonEqual(payload.Accounts, echoedAccounts) {
		return errPrepareSolanaStuckWithdrawEchoMismatch
	}
	return nil
}

func PrepareSolanaStuckWithdrawEnclaveCall(
	ctx context.Context,
	uk *cryptokeys.UserKeys,
	params types.PrepareSolanaStuckWithdrawParams,
) ([]types.PreparedStuckWithdrawJobType, error) {
	nullifyingKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	pair, err := uk.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	feeStructures := make(map[string]types.FeeStructureJSON, len(params.FeeStructures))
	for nullifierCount, feeStructure := range params.FeeStructures {
		feeStructures[nullifierCount] = types.FeeStructureJSON{
			FeeToken:     feeStructure.FeeToken,
			FlatFee:      feeStructure.FlatFee.String(),
			VariableRate: feeStructure.VariableRate.String(),
		}
	}
	payload := types.PrepareSolanaStuckWithdrawRequestType{
		ChainID:               strconv.Itoa(params.ChainID),
		MintAddress:           params.MintAddress,
		Recipient:             params.Recipient,
		RelayAddress:          params.RelayAddress,
		Accounts:              params.Accounts,
		FeeStructures:         feeStructures,
		HashedEthereumAddress: params.HashedEthereumAddress,
		NullifyingKey:         nullifyingKey,
		SpendingPublicKey:     [2]string{pair.PubSpendingBJJPoint[0].String(), pair.PubSpendingBJJPoint[1].String()},
	}

	resp, err := enclavePostSealedSigned[types.PrepareSolanaStuckWithdrawResponseType](ctx, constants.EnclaveConfig.PrepareSolanaStuckWithdraw, payload)
	if err != nil {
		return nil, err
	}
	if err := assertPrepareSolanaStuckWithdrawEchoMatches(payload, resp.Request); err != nil {
		return nil, err
	}
	for _, job := range resp.Jobs {
		nullifierCount := 0
		for _, amount := range job.InAmounts[0] {
			v, err := utils.ParseBigInt(amount)
			if err != nil {
				return nil, err
			}
			if v.Sign() > 0 {
				nullifierCount++
			}
		}
		feeStructure, ok := params.FeeStructures[strconv.Itoa(nullifierCount)]
		if !ok {
			return nil, fmt.Errorf("enclave: no feeStructure supplied for nullifierCount %d", nullifierCount)
		}
		if err := verifySolanaStuckWithdrawJobSignedMessageHash(params.MintAddress, params.RelayAddress, params.Recipient, feeStructure, nullifyingKey, payload.SpendingPublicKey, job); err != nil {
			return nil, err
		}
	}
	return resp.Jobs, nil
}
