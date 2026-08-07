package codec

import (
	"encoding/json"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func EncodeStealthAddressStructure(s types.StealthAddressStructure) types.StealthAddressStructureJSON {
	return types.StealthAddressStructureJSON{
		H0x:            EncodeBig(s.H0x),
		H0y:            EncodeBig(s.H0y),
		H1x:            EncodeBig(s.H1x),
		H1y:            EncodeBig(s.H1y),
		StealthAddress: EncodeBig(s.StealthAddress),
	}
}

func decodeStealthAddressStructure(s types.StealthAddressStructureJSON) (types.StealthAddressStructure, error) {
	h0x, err := DecodeBig(s.H0x)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h0y, err := DecodeBig(s.H0y)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h1x, err := DecodeBig(s.H1x)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h1y, err := DecodeBig(s.H1y)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	stealth, err := DecodeBig(s.StealthAddress)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	return types.StealthAddressStructure{
		H0x:            h0x,
		H0y:            h0y,
		H1x:            h1x,
		H1y:            h1y,
		StealthAddress: stealth,
	}, nil
}

func DecodeStealthAddressStructures(jsonArray string) ([]types.StealthAddressStructure, error) {
	var raw []types.StealthAddressStructureJSON
	if err := json.Unmarshal([]byte(jsonArray), &raw); err != nil {
		return nil, mobileerrors.InvalidJSON("stealthAddressStructuresJSON", err)
	}
	out := make([]types.StealthAddressStructure, len(raw))
	for i, r := range raw {
		decoded, err := decodeStealthAddressStructure(r)
		if err != nil {
			return nil, err
		}
		out[i] = decoded
	}
	return out, nil
}
