package codec

import (
	"encoding/json"
	"math/big"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
)

func JSONString(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DecodeBig(s string) (*big.Int, error) {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, mobileerrors.NotBase10(s)
	}
	return v, nil
}

func EncodeBig(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func DecodeBigs(jsonArray string) ([]*big.Int, error) {
	var raw []string
	if err := json.Unmarshal([]byte(jsonArray), &raw); err != nil {
		return nil, mobileerrors.InvalidJSON("amounts JSON", err)
	}
	out := make([]*big.Int, len(raw))
	for i, s := range raw {
		v, err := DecodeBig(s)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func DecodeStrings(jsonArray string) ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(jsonArray), &out); err != nil {
		return nil, mobileerrors.InvalidJSON("string array JSON", err)
	}
	return out, nil
}

func decodeInts(jsonArray string) ([]int, error) {
	var out []int
	if err := json.Unmarshal([]byte(jsonArray), &out); err != nil {
		return nil, mobileerrors.InvalidJSON("int array JSON", err)
	}
	return out, nil
}

func DecodeChainIDs(chainIDsJSON string) ([]int, error) {
	if chainIDsJSON == "" {
		return nil, nil
	}
	return decodeInts(chainIDsJSON)
}

func DecodeUserKeys(signature string) *cryptokeys.UserKeys {
	if signature == "" {
		return nil
	}
	return cryptokeys.NewUserKeys(signature)
}

func OptionalSeconds(sec int64) *int {
	if sec <= 0 {
		return nil
	}
	v := int(sec)
	return &v
}

func DecodePairedTokenAmounts(tokenAddrsJSON, amountsWeiJSON string) ([]string, []*big.Int, error) {
	tokenAddrs, err := DecodeStrings(tokenAddrsJSON)
	if err != nil {
		return nil, nil, err
	}
	amounts, err := DecodeBigs(amountsWeiJSON)
	if err != nil {
		return nil, nil, err
	}
	if len(tokenAddrs) == 0 || len(tokenAddrs) != len(amounts) {
		return nil, nil, mobileerrors.ErrTokensAmountsLengthMismatch
	}
	return tokenAddrs, amounts, nil
}

func DecodePairedAmountRecipients(amountsWeiJSON, recipientAddressesJSON string) ([]*big.Int, []string, error) {
	amounts, err := DecodeBigs(amountsWeiJSON)
	if err != nil {
		return nil, nil, err
	}
	recipients, err := DecodeStrings(recipientAddressesJSON)
	if err != nil {
		return nil, nil, err
	}
	if len(recipients) == 0 || len(recipients) != len(amounts) {
		return nil, nil, mobileerrors.ErrRecipientsAmountsLengthMismatch
	}
	return amounts, recipients, nil
}
