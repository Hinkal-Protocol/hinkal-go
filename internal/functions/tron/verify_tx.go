package tron

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

// txCheckWithArgs mirrors TronWeb's txCheckWithArgs for TriggerSmartContract
func txCheckWithArgs(tx *Transaction, owner, contract string, data []byte, callValueSun, feeLimitSun int64) error {
	raw, err := hex.DecodeString(tx.RawDataHex)
	if err != nil {
		return fmt.Errorf("decode raw_data_hex: %w", err)
	}
	var got core.TransactionRaw
	if err := proto.Unmarshal(raw, &got); err != nil {
		return fmt.Errorf("unmarshal raw_data_hex: %w", err)
	}
	ownerBytes, err := utils.TronAddressBytes(owner)
	if err != nil {
		return err
	}
	contractBytes, err := utils.TronAddressBytes(contract)
	if err != nil {
		return err
	}
	param, err := anypb.New(&core.TriggerSmartContract{
		OwnerAddress:    ownerBytes,
		ContractAddress: contractBytes,
		Data:            data,
		CallValue:       callValueSun,
	})
	if err != nil {
		return err
	}
	want, err := proto.MarshalOptions{Deterministic: true}.Marshal(&core.TransactionRaw{
		RefBlockBytes: got.RefBlockBytes,
		RefBlockHash:  got.RefBlockHash,
		Expiration:    got.Expiration,
		Timestamp:     got.Timestamp,
		FeeLimit:      feeLimitSun,
		Contract: []*core.Transaction_Contract{{
			Type:      core.Transaction_Contract_TriggerSmartContract,
			Parameter: param,
		}},
	})
	if err != nil {
		return err
	}
	txID := sha256.Sum256(want)
	if !bytes.Equal(want, raw) || !strings.EqualFold(tx.TxID, hex.EncodeToString(txID[:])) {
		return errors.New("invalid transaction")
	}
	return nil
}
