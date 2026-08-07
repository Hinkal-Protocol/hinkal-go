package solana

import (
	"encoding/base64"
	"encoding/binary"
	"strings"

	"github.com/mr-tron/base58"
)

const programDataPrefix = "Program data: "
const solanaOnChainFields = 9

var (
	discNewCommitment       = [8]byte{78, 21, 75, 243, 5, 132, 204, 67}
	discNullified           = [8]byte{124, 69, 200, 64, 75, 203, 121, 17}
	discBlockedUtxosCreated = [8]byte{64, 214, 216, 215, 75, 156, 106, 107}
)

type DecodedEvent struct {
	Name string
	Args map[string]any
}

func decodeEventData(full []byte) (*DecodedEvent, bool) {
	if len(full) < 8 {
		return nil, false
	}
	var disc [8]byte
	copy(disc[:], full[:8])
	fields := full[8:]
	switch disc {
	case discNewCommitment:
		return decodeNewCommitment(fields)
	case discNullified:
		return decodeNullified(fields)
	case discBlockedUtxosCreated:
		return &DecodedEvent{Name: "BlockedUtxosCreated", Args: map[string]any{}}, true
	default:
		return nil, false
	}
}

func decodeNewCommitment(b []byte) (*DecodedEvent, bool) {
	if len(b) < 32+32+4 {
		return nil, false
	}
	commitment := b[0:32]
	index := b[32:64]
	encLen := int(binary.LittleEndian.Uint32(b[64:68]))
	off := 68
	if len(b) < off+encLen+solanaOnChainFields*32 {
		return nil, false
	}
	encrypted := b[off : off+encLen]
	off += encLen
	onChain := make([][]int, solanaOnChainFields)
	for i := 0; i < solanaOnChainFields; i++ {
		onChain[i] = toIntSlice(b[off : off+32])
		off += 32
	}
	return &DecodedEvent{
		Name: "NewCommitment",
		Args: map[string]any{
			"commitment":       toIntSlice(commitment),
			"index":            toIntSlice(index),
			"encrypted_output": toIntSlice(encrypted),
			"on_chain_data":    onChain,
		},
	}, true
}

func decodeNullified(b []byte) (*DecodedEvent, bool) {
	if len(b) < 32 {
		return nil, false
	}
	return &DecodedEvent{
		Name: "Nullified",
		Args: map[string]any{"nullifier": toIntSlice(b[0:32])},
	}, true
}

func toIntSlice(b []byte) []int {
	out := make([]int, len(b))
	for i, v := range b {
		out[i] = int(v)
	}
	return out
}

func ParseLogsForEvents(logs []string) []*DecodedEvent {
	var out []*DecodedEvent
	for _, line := range logs {
		if !strings.HasPrefix(line, programDataPrefix) {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len(programDataPrefix):]))
		if err != nil {
			continue
		}
		if ev, ok := decodeEventData(raw); ok {
			out = append(out, ev)
		}
	}
	return out
}

func ParseCpiForEvents(instructionData []string) []*DecodedEvent {
	var out []*DecodedEvent
	for _, data := range instructionData {
		raw, err := base58.Decode(data)
		if err != nil || len(raw) <= 8 {
			continue
		}
		if ev, ok := decodeEventData(raw[8:]); ok {
			out = append(out, ev)
		}
	}
	return out
}
