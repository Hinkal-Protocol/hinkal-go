package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type WalletClient struct {
	fullHost   string
	walletHost string
}

func NewWalletClient(chainID int) (*WalletClient, error) {
	fullHost, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, err
	}
	walletHost, err := constants.TronWalletRPCURL(chainID)
	if err != nil {
		return nil, err
	}
	return &WalletClient{fullHost: fullHost, walletHost: walletHost}, nil
}

type Transaction struct {
	Visible    bool     `json:"visible"`
	TxID       string   `json:"txID"`
	RawData    any      `json:"raw_data,omitempty"`
	RawDataHex string   `json:"raw_data_hex"`
	Signature  []string `json:"signature,omitempty"`
}

type apiResult struct {
	Result  bool   `json:"result"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r apiResult) message() string {
	msg := r.Message
	if decoded, err := hex.DecodeString(msg); err == nil && len(decoded) > 0 {
		msg = string(decoded)
	}
	if msg == "" {
		msg = r.Code
	}
	return msg
}

type triggerResponse struct {
	Result      apiResult   `json:"result"`
	Transaction Transaction `json:"transaction"`
}

func (c *WalletClient) TriggerSmartContract(ctx context.Context, owner, contract string, data []byte, callValueSun, feeLimitSun int64) (*Transaction, error) {
	body := map[string]any{
		"owner_address":    owner,
		"contract_address": contract,
		"data":             hex.EncodeToString(data),
		"call_value":       callValueSun,
		"fee_limit":        feeLimitSun,
		"visible":          true,
	}
	var resp triggerResponse
	if err := api.Post(ctx, c.fullHost+"/wallet/triggersmartcontract", body, &resp); err != nil {
		return nil, fmt.Errorf("tron triggersmartcontract: %w", err)
	}
	if !resp.Result.Result {
		return nil, fmt.Errorf("tron triggersmartcontract rejected: %s", resp.Result.message())
	}
	if resp.Transaction.RawDataHex == "" {
		return nil, fmt.Errorf("tron triggersmartcontract returned empty transaction")
	}

	if err := txCheckWithArgs(&resp.Transaction, owner, contract, data, callValueSun, feeLimitSun); err != nil {
		return nil, fmt.Errorf("tron triggersmartcontract: %w", err)
	}
	return &resp.Transaction, nil
}

type constantContractResponse struct {
	Result      apiResult   `json:"result"`
	EnergyUsed  int64       `json:"energy_used"`
	Transaction Transaction `json:"transaction"`
}

func (c *WalletClient) TriggerConstantContract(ctx context.Context, owner, contract string, data []byte, callValueSun int64) (*constantContractResponse, error) {
	body := map[string]any{
		"owner_address":    owner,
		"contract_address": contract,
		"data":             hex.EncodeToString(data),
		"call_value":       callValueSun,
		"visible":          true,
	}
	var resp constantContractResponse
	if err := api.Post(ctx, c.fullHost+"/wallet/triggerconstantcontract", body, &resp); err != nil {
		return nil, fmt.Errorf("tron triggerconstantcontract: %w", err)
	}
	if !resp.Result.Result {
		return nil, fmt.Errorf("tron triggerconstantcontract rejected: %s", resp.Result.message())
	}
	return &resp, nil
}

type estimateEnergyResponse struct {
	Result         apiResult `json:"result"`
	EnergyRequired int64     `json:"energy_required"`
}

func (c *WalletClient) EstimateEnergy(ctx context.Context, owner, contract string, data []byte, callValueSun int64) (int64, error) {
	body := map[string]any{
		"owner_address":    owner,
		"contract_address": contract,
		"data":             hex.EncodeToString(data),
		"call_value":       callValueSun,
		"visible":          true,
	}
	var resp estimateEnergyResponse
	if err := api.Post(ctx, c.fullHost+"/wallet/estimateenergy", body, &resp); err != nil {
		return 0, fmt.Errorf("tron estimateenergy: %w", err)
	}
	if !resp.Result.Result {
		return 0, fmt.Errorf("tron estimateenergy rejected: %s", resp.Result.message())
	}
	return resp.EnergyRequired, nil
}

type ChainParameter struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

type chainParametersResponse struct {
	ChainParameter []ChainParameter `json:"chainParameter"`
}

func (c *WalletClient) GetChainParameters(ctx context.Context) ([]ChainParameter, error) {
	var resp chainParametersResponse
	if err := api.Post(ctx, c.walletHost+"/wallet/getchainparameters", map[string]any{}, &resp); err != nil {
		return nil, fmt.Errorf("tron getchainparameters: %w", err)
	}
	if len(resp.ChainParameter) == 0 {
		return nil, fmt.Errorf("tron getchainparameters returned no parameters")
	}
	return resp.ChainParameter, nil
}

type AccountResources struct {
	EnergyLimit  int64 `json:"EnergyLimit"`
	EnergyUsed   int64 `json:"EnergyUsed"`
	FreeNetLimit int64 `json:"freeNetLimit"`
	FreeNetUsed  int64 `json:"freeNetUsed"`
	NetLimit     int64 `json:"NetLimit"`
	NetUsed      int64 `json:"NetUsed"`
}

func (c *WalletClient) GetAccountResource(ctx context.Context, ownerHexPrefixed string) (*AccountResources, error) {
	body := map[string]any{
		"address": ownerHexPrefixed,
		"visible": false,
	}
	var resp AccountResources
	if err := api.Post(ctx, c.walletHost+"/wallet/getaccountresource", body, &resp); err != nil {
		return nil, fmt.Errorf("tron getaccountresource: %w", err)
	}
	return &resp, nil
}

type accountResponse struct {
	Balance int64 `json:"balance"`
}

func (c *WalletClient) GetAccountBalanceSun(ctx context.Context, owner string) (*big.Int, error) {
	body := map[string]any{
		"address": owner,
		"visible": true,
	}
	var resp accountResponse
	if err := api.Post(ctx, c.fullHost+"/wallet/getaccount", body, &resp); err != nil {
		return nil, fmt.Errorf("tron getaccount: %w", err)
	}
	return big.NewInt(resp.Balance), nil
}

func (c *WalletClient) BroadcastTransaction(ctx context.Context, tx *Transaction) (apiResult, error) {
	var resp apiResult
	if err := api.Post(ctx, c.fullHost+"/wallet/broadcasttransaction", tx, &resp); err != nil {
		return apiResult{}, fmt.Errorf("tron broadcasttransaction: %w", err)
	}
	return resp, nil
}

type TransactionInfo struct {
	ID          string `json:"id"`
	BlockNumber int64  `json:"blockNumber"`
	Result      string `json:"result"`
	ResMessage  string `json:"resMessage"`
}

func (c *WalletClient) GetTransactionInfoByID(ctx context.Context, txid string) (*TransactionInfo, error) {
	body := map[string]any{"value": txid}
	var resp TransactionInfo
	if err := api.Post(ctx, c.fullHost+"/wallet/gettransactioninfobyid", body, &resp); err != nil {
		return nil, fmt.Errorf("tron gettransactioninfobyid: %w", err)
	}
	return &resp, nil
}

type nowBlockResponse struct {
	BlockHeader struct {
		RawData struct {
			Number int64 `json:"number"`
		} `json:"raw_data"`
	} `json:"block_header"`
}

func (c *WalletClient) GetNowBlockNumber(ctx context.Context) (int64, error) {
	var resp nowBlockResponse
	if err := api.Post(ctx, c.fullHost+"/wallet/getnowblock", map[string]any{}, &resp); err != nil {
		return 0, fmt.Errorf("tron getnowblock: %w", err)
	}
	return resp.BlockHeader.RawData.Number, nil
}

type accountAssetsResponse struct {
	Data []struct {
		Balance int64               `json:"balance"`
		Trc20   []map[string]string `json:"trc20"`
	} `json:"data"`
}

const trc20TransferHistoryLimit = 200

type trc20TransferHistoryResponse struct {
	Data []struct {
		TokenInfo struct {
			Address string `json:"address"`
		} `json:"token_info"`
	} `json:"data"`
}

// getHeldTokenAddressesFromTransferHistory mirrors the TS getTronTokenAddressesFromTransferHistory
func (c *WalletClient) getHeldTokenAddressesFromTransferHistory(ctx context.Context, base58Address string) ([]string, error) {
	var resp trc20TransferHistoryResponse
	url := fmt.Sprintf(
		"%s/v1/accounts/%s/transactions/trc20?only_to=true&limit=%d",
		c.walletHost, base58Address, trc20TransferHistoryLimit,
	)
	if err := api.Get(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("tron get trc20 transfer history: %w", err)
	}

	seen := map[string]bool{}
	tokens := make([]string, 0, len(resp.Data))
	for _, transfer := range resp.Data {
		if transfer.TokenInfo.Address == "" {
			continue
		}
		hexAddress, err := utils.AddressToHexFormat(transfer.TokenInfo.Address)
		if err != nil || seen[hexAddress] {
			continue
		}
		seen[hexAddress] = true
		tokens = append(tokens, hexAddress)
	}
	return tokens, nil
}

// GetHeldTokenAddresses mirrors the TS getTronWalletTokenAddresses
func (c *WalletClient) GetHeldTokenAddresses(ctx context.Context, base58Address string) ([]string, error) {
	var resp accountAssetsResponse
	if err := api.Get(ctx, c.walletHost+"/v1/accounts/"+base58Address, &resp); err != nil {
		return nil, fmt.Errorf("tron get account assets: %w", err)
	}
	if len(resp.Data) == 0 {
		return c.getHeldTokenAddressesFromTransferHistory(ctx, base58Address)
	}
	account := resp.Data[0]

	tokens := make([]string, 0, len(account.Trc20)+1)
	if account.Balance > 0 {
		tokens = append(tokens, constants.ZeroAddress)
	}
	for _, entry := range account.Trc20 {
		for tokenBase58, balanceStr := range entry {
			balance, ok := new(big.Int).SetString(balanceStr, 10)
			if !ok || balance.Sign() <= 0 {
				continue
			}
			hexAddress, err := utils.AddressToHexFormat(tokenBase58)
			if err != nil {
				continue
			}
			tokens = append(tokens, hexAddress)
		}
	}
	return tokens, nil
}
