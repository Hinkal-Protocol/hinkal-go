package types

import (
	"encoding/json"
	"fmt"
)

// NewZkCallDataType mirrors the TS positional tuple
// [[a0,a1], [[b00,b01],[b10,b11]], [c0,c1], publicSignals]. It marshals to and
// from that JSON array so it round-trips with the enclave/relayer.
type NewZkCallDataType struct {
	A             [2]string
	B             [2][2]string
	C             [2]string
	PublicSignals []string
}

func (d NewZkCallDataType) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{d.A, d.B, d.C, d.PublicSignals})
}

func (d *NewZkCallDataType) UnmarshalJSON(data []byte) error {
	var raw [4]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("NewZkCallDataType: %w", err)
	}
	if err := json.Unmarshal(raw[0], &d.A); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &d.B); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &d.C); err != nil {
		return err
	}
	return json.Unmarshal(raw[3], &d.PublicSignals)
}
