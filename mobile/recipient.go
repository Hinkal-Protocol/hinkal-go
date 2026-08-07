package mobile

import (
	recipient "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/recipient"
)

func IsValidPrivateAddress(address string) bool {
	return recipient.IsValidPrivateAddress(address)
}

func NewStealthAddressStructureJSON(privateAddress string) (string, error) {
	return recipient.NewStealthAddressStructureJSON(privateAddress)
}

func GetRecipientInfoFromSignature(signature string) (string, error) {
	return recipient.GetRecipientInfoFromSignature(signature)
}

func GetStealthAddressStructureFromSignature(signature string) (string, error) {
	return recipient.GetStealthAddressStructureFromSignature(signature)
}
