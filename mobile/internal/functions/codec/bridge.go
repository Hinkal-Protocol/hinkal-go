package codec

import (
	"encoding/json"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

func EncodeBridgeQuote(q types.BridgeQuote) mobiletypes.BridgeQuoteJSON {
	return mobiletypes.BridgeQuoteJSON{
		Calldata:       q.Calldata,
		ExpectedAmount: EncodeBig(q.ExpectedAmount),
		NativeFee:      EncodeBig(q.NativeFee),
	}
}

func decodeBridgeRecipient(r mobiletypes.BridgeRecipientJSON) (types.BridgeRecipient, error) {
	amount, err := DecodeBig(r.BridgeAmount)
	if err != nil {
		return types.BridgeRecipient{}, err
	}
	expected, err := DecodeBig(r.Quote.ExpectedAmount)
	if err != nil {
		return types.BridgeRecipient{}, err
	}
	nativeFee, err := DecodeBig(r.Quote.NativeFee)
	if err != nil {
		return types.BridgeRecipient{}, err
	}
	return types.BridgeRecipient{
		RecipientAddress: r.RecipientAddress,
		BridgeAmount:     amount,
		Quote: types.BridgeQuote{
			Calldata:       r.Quote.Calldata,
			ExpectedAmount: expected,
			NativeFee:      nativeFee,
		},
		TemporarySubAccount: r.TemporarySubAccount,
	}, nil
}

func DecodeBridgeRecipients(recipientsJSON string) ([]types.BridgeRecipient, error) {
	var raw []mobiletypes.BridgeRecipientJSON
	if err := json.Unmarshal([]byte(recipientsJSON), &raw); err != nil {
		return nil, mobileerrors.InvalidJSON("recipientsJSON", err)
	}
	if len(raw) == 0 {
		return nil, mobileerrors.ErrEmptyRecipients
	}
	recipients := make([]types.BridgeRecipient, len(raw))
	for i, r := range raw {
		decoded, err := decodeBridgeRecipient(r)
		if err != nil {
			return nil, err
		}
		recipients[i] = decoded
	}
	return recipients, nil
}

func EncodeNearBridgeResult(res types.NearBridgeResult) (string, error) {
	legs := make([]mobiletypes.NearBridgeLegJSON, 0, len(res.Legs))
	for _, l := range res.Legs {
		legs = append(legs, mobiletypes.NearBridgeLegJSON{
			DestinationRecipient: l.DestinationRecipient,
			Amount:               EncodeBig(l.Amount),
			DepositAddress:       l.DepositAddress,
			Quote:                l.Quote,
		})
	}
	return JSONString(mobiletypes.NearBridgeResultJSON{DepositTxHash: res.DepositTxHash, Legs: legs})
}

func DecodePrivateBridgeRecipient(recipientJSON string) (bridging.PrivateBridgeRecipient, error) {
	var raw mobiletypes.PrivateBridgeRecipientJSON
	if err := json.Unmarshal([]byte(recipientJSON), &raw); err != nil {
		return bridging.PrivateBridgeRecipient{}, mobileerrors.InvalidJSON("recipientJSON", err)
	}
	out := bridging.PrivateBridgeRecipient{
		RecipientInfo:       raw.RecipientInfo,
		RecipientEthAddress: raw.RecipientEthAddress,
	}
	if raw.ClaimableNonce != "" {
		nonce, err := DecodeBig(raw.ClaimableNonce)
		if err != nil {
			return bridging.PrivateBridgeRecipient{}, err
		}
		out.ClaimableNonce = nonce
	}
	return out, nil
}
