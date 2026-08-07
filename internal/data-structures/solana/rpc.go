package solana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Client struct {
	url    string
	client *http.Client
}

func NewClient(url string) *Client {
	return &Client{url: url, client: &http.Client{}}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) call(ctx context.Context, method string, params []any, out any) (err error) {
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	var r rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}
	if r.Error != nil {
		return fmt.Errorf("rpc %s: %d %s", method, r.Error.Code, r.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

func (c *Client) GetSlot(ctx context.Context) (uint64, error) {
	var slot uint64
	err := c.call(ctx, "getSlot", []any{map[string]any{"commitment": "confirmed"}}, &slot)
	return slot, err
}

func (c *Client) GetAccountInfo(ctx context.Context, address string) ([]byte, error) {
	var result struct {
		Value *struct {
			Data []string `json:"data"`
		} `json:"value"`
	}
	params := []any{address, map[string]any{"commitment": "confirmed", "encoding": "base64"}}
	if err := c.call(ctx, "getAccountInfo", params, &result); err != nil {
		return nil, err
	}
	if result.Value == nil {
		return nil, fmt.Errorf("getAccountInfo: account %s not found", address)
	}
	if len(result.Value.Data) == 0 {
		return nil, fmt.Errorf("getAccountInfo: account %s has no data", address)
	}
	return base64.StdEncoding.DecodeString(result.Value.Data[0])
}

type SignatureInfo struct {
	Signature string `json:"signature"`
	Slot      uint64 `json:"slot"`
	Err       any    `json:"err"`
}

func (c *Client) GetSignaturesForAddress(ctx context.Context, address string, limit int, before string) ([]SignatureInfo, error) {
	opts := map[string]any{"limit": limit, "commitment": "confirmed"}
	if before != "" {
		opts["before"] = before
	}
	var sigs []SignatureInfo
	err := c.call(ctx, "getSignaturesForAddress", []any{address, opts}, &sigs)
	return sigs, err
}

type Instruction struct {
	Data string `json:"data"`
}

type InnerInstruction struct {
	Instructions []Instruction `json:"instructions"`
}

type TxMeta struct {
	Err               any                `json:"err"`
	LogMessages       []string           `json:"logMessages"`
	InnerInstructions []InnerInstruction `json:"innerInstructions"`
}

type Transaction struct {
	Slot      uint64  `json:"slot"`
	BlockTime *int64  `json:"blockTime"`
	Meta      *TxMeta `json:"meta"`
}

func (c *Client) GetTransaction(ctx context.Context, signature string) (*Transaction, error) {
	opts := map[string]any{"commitment": "confirmed", "maxSupportedTransactionVersion": 0, "encoding": "json"}
	var tx *Transaction
	err := c.call(ctx, "getTransaction", []any{signature, opts}, &tx)
	return tx, err
}
