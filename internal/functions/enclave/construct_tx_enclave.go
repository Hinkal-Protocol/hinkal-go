package enclave

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errPrepareTxEchoMismatch            = errors.New("enclave: prepare-tx response does not match the request sent - refusing to sign")
	errPrepareStuckWithdrawEchoMismatch = errors.New("enclave: prepare-stuck-withdraw response does not match the request sent - refusing to sign")
)

func enclavePost[T any](ctx context.Context, path string, payload any) (T, error) {
	var dest T
	body, err := json.Marshal(payload)
	if err != nil {
		return dest, err
	}
	keyCiphertext, inputCiphertext, err := MakeHandshakeAndEncrypt(ctx, body)
	if err != nil {
		return dest, err
	}
	var signed types.SignedEnclaveResponse
	reqBody := map[string]any{"key": keyCiphertext, "inputs": inputCiphertext}
	if err := api.Post(ctx, constants.GetEnclaveURL()+path, reqBody, &signed); err != nil {
		return dest, err
	}
	return verifySignedEnclaveResponse[T](signed.Data, signed.Signature)
}

func bigIntStrings(values []*big.Int) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

func buildPrepareTxRequest(p types.PrepareTxParams, nullifyingKey string, spendingPublicKey [2]string) types.PrepareTxRequestType {
	req := types.PrepareTxRequestType{
		ChainID:                strconv.Itoa(p.ChainID),
		Erc20Addresses:         p.Erc20Addresses,
		AmountChanges:          bigIntStrings(p.AmountChanges),
		ExternalAddress:        p.ExternalAddress,
		OriginalSender:         p.OriginalSender,
		Relay:                  p.Relay,
		ExternalActionID:       string(p.ExternalActionID),
		ExternalActionMetadata: p.ExternalActionMetadata,
		OnChainCreation:        p.OnChainCreation,
		RecipientAddress:       p.RecipientAddress,
		RecipientAmounts:       bigIntStrings(p.RecipientAmounts),
		InputCommitments:       p.InputCommitments,
		UseBlockedUtxos:        p.UseBlockedUtxos,
		ForceEmptyUtxos:        p.ForceEmptyUtxos,
		SkipLock:               p.SkipLock,
		NullifyingKey:          nullifyingKey,
		SpendingPublicKey:      spendingPublicKey,
	}
	if p.FeeStructure != nil {
		req.FeeStructure = &types.FeeStructureJSON{
			FeeToken:     p.FeeStructure.FeeToken,
			FlatFee:      p.FeeStructure.FlatFee.String(),
			VariableRate: p.FeeStructure.VariableRate.String(),
		}
	}
	if p.MessageSeed != nil {
		req.MessageSeed = p.MessageSeed.String()
	}
	return req
}

func assertPrepareTxEchoMatches(payload types.PrepareTxRequestType, echoed types.EchoedPrepareTxRequestType) error {
	expected := types.EchoedPrepareTxRequestType{
		ChainID:                payload.ChainID,
		Erc20Addresses:         payload.Erc20Addresses,
		AmountChanges:          payload.AmountChanges,
		ExternalAddress:        payload.ExternalAddress,
		OriginalSender:         payload.OriginalSender,
		Relay:                  payload.Relay,
		FeeStructure:           payload.FeeStructure,
		ExternalActionID:       payload.ExternalActionID,
		ExternalActionMetadata: payload.ExternalActionMetadata,
		OnChainCreation:        payload.OnChainCreation,
		RecipientAddress:       payload.RecipientAddress,
		RecipientAmounts:       payload.RecipientAmounts,
		InputCommitments:       payload.InputCommitments,
		UseBlockedUtxos:        payload.UseBlockedUtxos,
		ForceEmptyUtxos:        payload.ForceEmptyUtxos,
		SkipLock:               payload.SkipLock,
		MessageSeed:            payload.MessageSeed,
	}
	if !reflect.DeepEqual(expected, echoed) {
		return errPrepareTxEchoMismatch
	}
	return nil
}

func assertPrepareStuckWithdrawEchoMatches(payload types.PrepareStuckWithdrawRequestType, echoed types.EchoedPrepareStuckWithdrawRequestType) error {
	expected := types.EchoedPrepareStuckWithdrawRequestType{
		ChainID:               payload.ChainID,
		Erc20Address:          payload.Erc20Address,
		ExternalAddress:       payload.ExternalAddress,
		Relay:                 payload.Relay,
		FeeStructure:          payload.FeeStructure,
		HashedEthereumAddress: payload.HashedEthereumAddress,
	}
	if !reflect.DeepEqual(expected, echoed) {
		return errPrepareStuckWithdrawEchoMismatch
	}
	return nil
}

func PrepareTxEnclaveCall(ctx context.Context, uk *cryptokeys.UserKeys, params types.PrepareTxParams) (types.PrepareTxResponseType, error) {
	nullifyingKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return types.PrepareTxResponseType{}, err
	}
	pair, err := uk.GetSpendingKeyPair()
	if err != nil {
		return types.PrepareTxResponseType{}, err
	}
	spendingPublicKey := [2]string{pair.PubSpendingBJJPoint[0].String(), pair.PubSpendingBJJPoint[1].String()}
	payload := buildPrepareTxRequest(params, nullifyingKey, spendingPublicKey)

	resp, err := enclavePost[types.PrepareTxResponseType](ctx, constants.EnclaveConfig.PrepareTx, payload)
	if err != nil {
		return types.PrepareTxResponseType{}, err
	}
	if err := assertPrepareTxEchoMatches(payload, resp.Request); err != nil {
		return types.PrepareTxResponseType{}, err
	}
	return resp, nil
}

type PrepareStuckWithdrawParams struct {
	ChainID               int
	Erc20Address          string
	ExternalAddress       string
	Relay                 string
	FeeStructure          types.FeeStructure
	HashedEthereumAddress string
}

func PrepareStuckWithdrawEnclaveCall(ctx context.Context, uk *cryptokeys.UserKeys, params PrepareStuckWithdrawParams) ([]types.PreparedJobType, error) {
	nullifyingKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	pair, err := uk.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	payload := types.PrepareStuckWithdrawRequestType{
		ChainID:         strconv.Itoa(params.ChainID),
		Erc20Address:    params.Erc20Address,
		ExternalAddress: params.ExternalAddress,
		Relay:           params.Relay,
		FeeStructure: types.FeeStructureJSON{
			FeeToken:     params.FeeStructure.FeeToken,
			FlatFee:      params.FeeStructure.FlatFee.String(),
			VariableRate: params.FeeStructure.VariableRate.String(),
		},
		HashedEthereumAddress: params.HashedEthereumAddress,
		NullifyingKey:         nullifyingKey,
		SpendingPublicKey:     [2]string{pair.PubSpendingBJJPoint[0].String(), pair.PubSpendingBJJPoint[1].String()},
	}
	resp, err := enclavePost[types.PrepareStuckWithdrawResponseType](ctx, constants.EnclaveConfig.PrepareStuckWithdraw, payload)
	if err != nil {
		return nil, err
	}
	if err := assertPrepareStuckWithdrawEchoMatches(payload, resp.Request); err != nil {
		return nil, err
	}
	return resp.Jobs, nil
}

func eddsaSignatureWire(sig types.EddsaSignature) types.EddsaSignatureWire {
	return types.EddsaSignatureWire{
		R8: [2]string{sig.R8[0].String(), sig.R8[1].String()},
		S:  sig.S.String(),
	}
}

func FinalizeTxEnclaveCall(ctx context.Context, jobID string, sig types.EddsaSignature) (types.FinalizeTxResponseType, error) {
	payload := types.FinalizeTxRequestType{
		JobID:          jobID,
		EddsaSignature: eddsaSignatureWire(sig),
	}
	return enclavePost[types.FinalizeTxResponseType](ctx, constants.EnclaveConfig.FinalizeTx, payload)
}

type FinalizeTxRelayExtra struct {
	AdminData             *types.AdminDataType
	AuthorizationData     *types.AuthorizationData
	WithUniswapWorkAround *bool
}

func FinalizeTxEnclaveCallRelay(ctx context.Context, jobID string, sig types.EddsaSignature, chainID int, extra FinalizeTxRelayExtra) (string, error) {
	payload := types.FinalizeTxRelayRequestType{
		JobID:                 jobID,
		EddsaSignature:        eddsaSignatureWire(sig),
		ChainID:               strconv.Itoa(chainID),
		AdminData:             extra.AdminData,
		AuthorizationData:     extra.AuthorizationData,
		WithUniswapWorkAround: extra.WithUniswapWorkAround,
	}
	resp, err := enclavePost[types.FinalizeTxRelayResponseType](ctx, constants.EnclaveConfig.FinalizeTxRelay, payload)
	if err != nil {
		return "", err
	}
	return resp.TxHash, nil
}
