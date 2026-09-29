package solana

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"slices"
	"strings"

	"github.com/mr-tron/base58"
)

const (
	programDataPrefix   = "Program data: "
	programLinePrefix   = "Program "
	solanaOnChainFields = 9
	minProgramIDLength  = 32
	maxProgramIDLength  = 44
)

var (
	discNewCommitment       = [8]byte{78, 21, 75, 243, 5, 132, 204, 67}
	discNullified           = [8]byte{124, 69, 200, 64, 75, 203, 121, 17}
	discBlockedUtxosCreated = [8]byte{64, 214, 216, 215, 75, 156, 106, 107}
	anchorEventIxTagLE      = []byte{0xe4, 0x45, 0xa5, 0x2e, 0x51, 0xcb, 0x9a, 0x1d}
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

func ParseTransactionEvents(tx *Transaction, programID string) []*DecodedEvent {
	if tx == nil || tx.Meta == nil {
		return nil
	}
	return append(parseProgramLogs(tx.Meta.LogMessages, programID), parseProgramCpi(tx, programID)...)
}

func isProgramID(token string) bool {
	return len(token) >= minProgramIDLength && len(token) <= maxProgramIDLength
}

func parseProgramLogs(logs []string, programID string) []*DecodedEvent {
	var out []*DecodedEvent
	var invokeStack []string
	for _, line := range logs {
		if encoded, ok := strings.CutPrefix(line, programDataPrefix); ok {
			if len(invokeStack) == 0 || invokeStack[len(invokeStack)-1] != programID {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
			if err != nil {
				continue
			}
			if ev, ok := decodeEventData(raw); ok {
				out = append(out, ev)
			}
			continue
		}
		rest, ok := strings.CutPrefix(line, programLinePrefix)
		if !ok {
			continue
		}
		id, action, ok := strings.Cut(rest, " ")
		if !ok || !isProgramID(id) {
			continue
		}
		switch {
		case strings.HasPrefix(action, "invoke ["):
			invokeStack = append(invokeStack, id)
		case (strings.HasPrefix(action, "success") || strings.HasPrefix(action, "failed")) && len(invokeStack) > 0:
			invokeStack = invokeStack[:len(invokeStack)-1]
		}
	}
	return out
}

func parseProgramCpi(tx *Transaction, programID string) []*DecodedEvent {
	keys := append(append(slices.Clone(tx.Transaction.Message.AccountKeys),
		tx.Meta.LoadedAddresses.Writable...), tx.Meta.LoadedAddresses.Readonly...)
	var out []*DecodedEvent
	for _, inner := range tx.Meta.InnerInstructions {
		for _, ix := range inner.Instructions {
			if ix.ProgramIDIndex < 0 || ix.ProgramIDIndex >= len(keys) || keys[ix.ProgramIDIndex] != programID {
				continue
			}
			raw, err := base58.Decode(ix.Data)
			if err != nil || len(raw) <= len(anchorEventIxTagLE) || !bytes.HasPrefix(raw, anchorEventIxTagLE) {
				continue
			}
			if ev, ok := decodeEventData(raw[len(anchorEventIxTagLE):]); ok {
				out = append(out, ev)
			}
		}
	}
	return out
}
