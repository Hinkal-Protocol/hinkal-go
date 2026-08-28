package tron

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type ethCallResponse struct {
	Result string `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
}

// ethCall runs eth_call against the Tron node JSON-RPC and returns the result hex, or an error if
// the call reverts. from/to are 0x-hex addresses.
func ethCall(ctx context.Context, chainID int, from, to string, data []byte, value *big.Int) (string, error) {
	rpcURL, err := constants.TronJSONRPCURL(chainID)
	if err != nil {
		return "", err
	}
	callObject := map[string]any{
		"from": from,
		"to":   to,
		"data": "0x" + common.Bytes2Hex(data),
	}
	if value != nil && value.Sign() > 0 {
		callObject["value"] = "0x" + value.Text(16)
	}
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_call",
		"params":  []any{callObject, "latest"},
	}

	var resp ethCallResponse
	if err := api.Post(ctx, rpcURL, body, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		message := resp.Error.Message
		if message == "" {
			message = "Unknown revert"
		}
		return "", fmt.Errorf("tron eth_call reverted: %s", message)
	}
	return resp.Result, nil
}

// SimulateTronTransaction runs the call through the Tron node's eth_call endpoint and returns an
// error if it reverts. callData is the ABI-encoded input (e.g. from contractabi.PackTronTransact).
func SimulateTronTransaction(ctx context.Context, chainID int, contractAddress, from string, callData []byte, value *big.Int) error {
	contractHex, err := utils.AddressToHexFormat(contractAddress)
	if err != nil {
		return err
	}
	fromHex, err := utils.AddressToHexFormat(from)
	if err != nil {
		return err
	}
	_, err = ethCall(ctx, chainID, fromHex, contractHex, callData, value)
	if err != nil {
		return fmt.Errorf("tron transaction simulation reverted: %w", err)
	}
	return nil
}

func FetchRootHash(ctx context.Context, chainID int) (*big.Int, error) {
	hinkalHex, err := constants.HinkalAddress(chainID)
	if err != nil {
		return nil, err
	}
	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	callData, err := parsedABI.Pack("getRootHash")
	if err != nil {
		return nil, err
	}
	resultHex, err := ethCall(ctx, chainID, constants.ZeroAddress, hinkalHex, callData, nil)
	if err != nil {
		return nil, err
	}
	results, err := parsedABI.Unpack("getRootHash", common.FromHex(resultHex))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("tron getRootHash: empty result")
	}
	rootHash, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("tron getRootHash: unexpected type %T", results[0])
	}
	return rootHash, nil
}
