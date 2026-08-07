package recipient

import (
	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func IsValidPrivateAddress(address string) bool {
	return pretransaction.IsValidPrivateAddress(address)
}

func NewStealthAddressStructureJSON(privateAddress string) (string, error) {
	if !pretransaction.IsValidPrivateAddress(privateAddress) {
		return "", mobileerrors.ErrInvalidPrivateAddress
	}
	s, err := pretransaction.ConstructStealthAddressStructure(privateAddress)
	if err != nil {
		return "", err
	}
	return codec.JSONString(codec.EncodeStealthAddressStructure(s))
}

func GetRecipientInfoFromSignature(signature string) (string, error) {
	userKeys := codec.DecodeUserKeys(signature)
	if userKeys == nil {
		return "", mobileerrors.ErrSignatureRequired
	}
	return pretransaction.GetRecipientInfoFromUserKeys(userKeys)
}

func GetStealthAddressStructureFromSignature(signature string) (string, error) {
	userKeys := codec.DecodeUserKeys(signature)
	if userKeys == nil {
		return "", mobileerrors.ErrSignatureRequired
	}
	s, err := pretransaction.GetStealthAddressStructureFromUserKeys(userKeys)
	if err != nil {
		return "", err
	}
	return codec.JSONString(codec.EncodeStealthAddressStructure(s))
}
