package blockchainevent

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
)

type EventProcessorFunc func(events []*BlockchainEvent, scannedToBlock *uint64) (int, error)

type BlockchainEvent struct {
	EventName       string
	TransactionHash string
	BlockNumber     uint64
	args            string
}

func NewFromLog(log ethtypes.Log, contractABI abi.ABI) (*BlockchainEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}
	event, err := contractABI.EventByID(log.Topics[0])
	if err != nil {
		return nil, fmt.Errorf("unknown event topic %s: %w", log.Topics[0].Hex(), err)
	}
	argsMap := make(map[string]any)
	nonIndexed := event.Inputs.NonIndexed()
	if len(log.Data) > 0 && len(nonIndexed) > 0 {
		vals, err := nonIndexed.Unpack(log.Data)
		if err != nil {
			return nil, fmt.Errorf("unpack log data: %w", err)
		}
		ni := 0
		for i, input := range event.Inputs {
			if input.Indexed {
				continue
			}
			if ni < len(vals) {
				argsMap[fmt.Sprintf("%d", i)] = normalizeABIValue(vals[ni])
				argsMap[input.Name] = normalizeABIValue(vals[ni])
				ni++
			}
		}
	}
	var indexedInputs abi.Arguments
	for _, input := range event.Inputs {
		if input.Indexed {
			indexedInputs = append(indexedInputs, input)
		}
	}
	if len(indexedInputs) > 0 && len(log.Topics) > 1 {
		indexedMap := make(map[string]any)
		if err := abi.ParseTopicsIntoMap(indexedMap, indexedInputs, log.Topics[1:]); err != nil {
			return nil, fmt.Errorf("parse indexed topics: %w", err)
		}
		for i, input := range event.Inputs {
			if !input.Indexed {
				continue
			}
			v := indexedMap[input.Name]
			argsMap[fmt.Sprintf("%d", i)] = normalizeABIValue(v)
			argsMap[input.Name] = normalizeABIValue(v)
		}
	}
	b, err := json.Marshal(argsMap)
	if err != nil {
		return nil, err
	}
	return &BlockchainEvent{
		EventName:       event.Name,
		TransactionHash: log.TxHash.Hex(),
		BlockNumber:     log.BlockNumber,
		args:            string(b),
	}, nil
}

func NewFromSerialized(serialized string) (*BlockchainEvent, error) {
	var raw struct {
		EventName       string `json:"eventName"`
		TransactionHash string `json:"transactionHash"`
		BlockNumber     uint64 `json:"blockNumber"`
		Args            any    `json:"args"`
	}
	if err := json.Unmarshal([]byte(serialized), &raw); err != nil {
		return nil, err
	}
	var argsStr string
	switch a := raw.Args.(type) {
	case string:
		argsStr = a
	default:
		b, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		argsStr = string(b)
	}
	return &BlockchainEvent{
		EventName:       raw.EventName,
		TransactionHash: raw.TransactionHash,
		BlockNumber:     raw.BlockNumber,
		args:            argsStr,
	}, nil
}

func NewFromSolanaEvent(d *solana.DecodedEvent, signature string, slot uint64) (*BlockchainEvent, error) {
	argsJSON, err := json.Marshal(d.Args)
	if err != nil {
		return nil, err
	}
	return &BlockchainEvent{
		EventName:       d.Name,
		TransactionHash: signature,
		BlockNumber:     slot,
		args:            string(argsJSON),
	}, nil
}

func (e *BlockchainEvent) GetArg(key string) (string, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(e.args), &m); err != nil {
		return "", err
	}
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("arg %q not found in event %s", key, e.EventName)
	}
	switch val := v.(type) {
	case string:
		return val, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func (e *BlockchainEvent) Serialize() (string, error) {
	m := map[string]any{
		"eventName":       e.EventName,
		"transactionHash": e.TransactionHash,
		"blockNumber":     e.BlockNumber,
		"args":            e.args,
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func normalizeABIValue(v any) any {
	switch val := v.(type) {
	case *big.Int:
		if val == nil {
			return "0"
		}
		return val.String()
	case common.Address:
		return val.Hex()
	case []byte:
		return "0x" + common.Bytes2Hex(val)
	case [32]byte:
		return "0x" + common.Bytes2Hex(val[:])
	default:
		return v
	}
}
