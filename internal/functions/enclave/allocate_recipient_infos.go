package enclave

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type PrivateSendAllocation struct {
	RecipientAddress string `json:"recipientAddress"`
	RecipientInfo    string `json:"recipientInfo,omitempty"`
}

var errAllocateRecipientInfosEchoMismatch = errors.New("enclave: allocate-recipient-infos response does not match the request sent - refusing to use it")

type allocateRecipientInfosResponse struct {
	Allocations []PrivateSendAllocation `json:"allocations"`
}

func allocateRecipientInfosEnclaveCall(ctx context.Context, recipientAddresses []string) (allocateRecipientInfosResponse, error) {
	handshake, err := MakeHandshakeAndEncrypt(ctx, nil)
	if err != nil {
		return allocateRecipientInfosResponse{}, err
	}
	var raw json.RawMessage
	if err := api.Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.AllocateRecipientInfos, map[string]any{
		"key":                handshake.KeyCiphertext,
		"recipientAddresses": recipientAddresses,
	}, &raw); err != nil {
		return allocateRecipientInfosResponse{}, err
	}
	signed, err := OpenSealedResponse[types.SignedEnclaveResponse](raw, handshake.Key)
	if err != nil {
		return allocateRecipientInfosResponse{}, err
	}
	response, err := verifySignedEnclaveResponse[allocateRecipientInfosResponse](signed.Data, signed.Signature)
	if err != nil {
		return allocateRecipientInfosResponse{}, err
	}

	echoedMatches := len(response.Allocations) == len(recipientAddresses)
	for index, allocation := range response.Allocations {
		if !echoedMatches {
			break
		}
		echoedMatches = allocation.RecipientAddress == recipientAddresses[index]
	}
	if !echoedMatches {
		return allocateRecipientInfosResponse{}, errAllocateRecipientInfosEchoMismatch
	}

	return response, nil
}

func AllocateRecipientInfos(ctx context.Context, recipientAddresses []string) ([]PrivateSendAllocation, error) {
	response, err := allocateRecipientInfosEnclaveCall(ctx, recipientAddresses)
	if err != nil {
		return nil, err
	}
	return response.Allocations, nil
}
