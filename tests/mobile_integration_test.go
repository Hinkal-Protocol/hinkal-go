package tests

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/mobile"
)

const (
	e2eEthAddress = "0x1111111111111111111111111111111111111111"
	e2eUSDCBase   = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
)

func e2eConnectedClient(t *testing.T) *mobile.Client {
	t.Helper()
	c := mobile.NewClient()
	if _, err := c.Connect(fakeMobileHost{
		sig:   mobileFixedSignature,
		addr:  e2eEthAddress,
		chain: constants.ChainIDs.Base,
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Disconnect(); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	})
	return c
}

func e2eConnectedHinkal(t *testing.T) *mobile.Hinkal {
	t.Helper()
	return e2eConnectedClient(t).Hinkal()
}

func mustJSONArray(t *testing.T, label, raw string) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("%s is not a JSON array (%q): %v", label, raw, err)
	}
	return out
}

func TestMobileIntegrationTokenConversionRoundTrips(t *testing.T) {
	requireLive(t)
	chainID := int64(constants.ChainIDs.Base)
	token := envOr("HINKAL_E2E_TOKEN", e2eUSDCBase)

	wei, err := mobile.AmountToWei(chainID, token, "1.5")
	if err != nil {
		t.Fatalf("AmountToWei: %v", err)
	}
	back, err := mobile.AmountFromWei(chainID, token, wei)
	if err != nil {
		t.Fatalf("AmountFromWei: %v", err)
	}
	if strings.TrimRight(strings.TrimRight(back, "0"), ".") != "1.5" {
		t.Fatalf("1.5 -> %s wei -> %s, want 1.5 back", wei, back)
	}

	if _, err := mobile.AmountWithPrecision(chainID, token, wei, 2); err != nil {
		t.Fatalf("AmountWithPrecision: %v", err)
	}
}

func TestMobileIntegrationTokenListAndBalances(t *testing.T) {
	requireLive(t)
	chainID := int64(constants.ChainIDs.Base)

	h := e2eConnectedHinkal(t)

	raw, err := mobile.GetTokensJSON(chainID)
	if err != nil {
		t.Fatalf("GetTokensJSON: %v", err)
	}
	if len(mustJSONArray(t, "GetTokensJSON", raw)) == 0 {
		t.Fatal("GetTokensJSON returned no tokens")
	}

	balances, err := h.GetTotalBalance(chainID, "", "", true, false)
	if err != nil {
		t.Fatalf("GetTotalBalance: %v", err)
	}
	var parsed []struct {
		Token struct {
			Erc20TokenAddress string `json:"erc20TokenAddress"`
			ChainID           int    `json:"chainId"`
			Decimals          int    `json:"decimals"`
		} `json:"token"`
		Balance string `json:"balance"`
	}
	if err := json.Unmarshal([]byte(balances), &parsed); err != nil {
		t.Fatalf("bad balances JSON %q: %v", balances, err)
	}
	for _, b := range parsed {
		if strings.ContainsAny(b.Balance, "eE.") {
			t.Errorf("balance %q for %s is not a plain base-10 string", b.Balance, b.Token.Erc20TokenAddress)
		}
		if b.Token.Erc20TokenAddress == "" {
			t.Error("balance entry has no token address; nested token shape may be wrong")
		}
	}
}

func TestMobileIntegrationFeeStructure(t *testing.T) {
	requireLive(t)
	chainID := int64(constants.ChainIDs.Base)
	token := envOr("HINKAL_E2E_TOKEN", e2eUSDCBase)

	c := e2eConnectedClient(t)
	raw, err := c.GetFeeStructureJSON(chainID, token, `["`+token+`"]`, "", "", "", "")
	if err != nil {
		t.Fatalf("GetFeeStructureJSON: %v", err)
	}
	var fee struct {
		FeeToken     string `json:"feeToken"`
		FlatFee      string `json:"flatFee"`
		VariableRate string `json:"variableRate"`
	}
	if err := json.Unmarshal([]byte(raw), &fee); err != nil {
		t.Fatalf("bad fee JSON %q: %v", raw, err)
	}
	if fee.FeeToken == "" {
		t.Error("fee structure has no feeToken")
	}
	for name, v := range map[string]string{"flatFee": fee.FlatFee, "variableRate": fee.VariableRate} {
		if v == "" || strings.ContainsAny(v, "eE.") {
			t.Errorf("%s = %q, want a plain base-10 string", name, v)
		}
	}
}

func TestMobileIntegrationSwapQuotes(t *testing.T) {
	requireLive(t)
	chainID := int64(constants.ChainIDs.Base)
	in := envOr("HINKAL_E2E_TOKEN", e2eUSDCBase)
	out := envOr("HINKAL_E2E_TOKEN_OUT", "0x4200000000000000000000000000000000000006")

	raw, err := mobile.GetSwapQuotesJSON(chainID, "1", in, out)
	if err != nil {
		t.Fatalf("GetSwapQuotesJSON: %v", err)
	}
	var parsed []struct {
		ActionID  string `json:"actionId"`
		OutAmount string `json:"outAmount"`
		SwapData  string `json:"swapData"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("bad quotes JSON %q: %v", raw, err)
	}
	for _, q := range parsed {
		if q.ActionID == "" || q.OutAmount == "" {
			t.Errorf("incomplete quote: %+v", q)
		}
		if strings.ContainsAny(q.OutAmount, "eE.") {
			t.Errorf("outAmount %q is not a plain base-10 string", q.OutAmount)
		}
	}
}

func TestMobileIntegrationDepositAndWithdraw(t *testing.T) {
	requireLive(t)
	chainID := int64(constants.ChainIDs.Base)
	token := envOr("HINKAL_E2E_TOKEN", e2eUSDCBase)
	privateKey := envOr("HINKAL_PRIVATE_KEY", "")
	if privateKey == "" {
		t.Skip("set HINKAL_PRIVATE_KEY to run the deposit/withdraw money path")
	}

	c := mobile.NewClient()
	h := c.Hinkal()
	if _, err := c.ConnectWithPrivateKey(privateKey, chainID); err != nil {
		t.Fatalf("ConnectWithPrivateKey: %v", err)
	}
	t.Cleanup(func() { _ = h.Destroy() })

	amount := envOr("HINKAL_E2E_AMOUNT", "0.1")
	wei, err := mobile.AmountToWei(chainID, token, amount)
	if err != nil {
		t.Fatalf("AmountToWei: %v", err)
	}

	depositRaw, err := h.Deposit(chainID, `["`+token+`"]`, `["`+wei+`"]`, false, false)
	if err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	var deposit struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal([]byte(depositRaw), &deposit); err != nil {
		t.Fatalf("bad Deposit JSON %q: %v", depositRaw, err)
	}
	t.Logf("deposit tx: %s", deposit.Hash)
	if ok, err := h.WaitForTransaction(chainID, deposit.Hash, 1, 900); err != nil || !ok {
		t.Fatalf("deposit not confirmed: ok=%v err=%v", ok, err)
	}
	if err := h.ResetMerkle(`[` + strconv.FormatInt(chainID, 10) + `]`); err != nil {
		t.Fatalf("ResetMerkle: %v", err)
	}

	addr, err := h.GetEthereumAddressByChain(chainID)
	if err != nil {
		t.Fatalf("GetEthereumAddressByChain: %v", err)
	}
	withdrawRaw, err := h.Withdraw(chainID, `["`+token+`"]`, `["-`+wei+`"]`, addr, false, "", "")
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	t.Logf("withdraw tx: %s", withdrawRaw)
}

func TestMobileIntegrationScheduledSendIsPollable(t *testing.T) {
	requireLive(t)
	chainID := int64(constants.ChainIDs.Base)
	token := envOr("HINKAL_E2E_TOKEN", e2eUSDCBase)
	privateKey := envOr("HINKAL_PRIVATE_KEY", "")
	if privateKey == "" {
		t.Skip("set HINKAL_PRIVATE_KEY to run the scheduled-send money path")
	}

	c := mobile.NewClient()
	h := c.Hinkal()
	if _, err := c.ConnectWithPrivateKey(privateKey, chainID); err != nil {
		t.Fatalf("ConnectWithPrivateKey: %v", err)
	}
	t.Cleanup(func() { _ = h.Destroy() })

	addr, err := h.GetEthereumAddressByChain(chainID)
	if err != nil {
		t.Fatalf("GetEthereumAddressByChain: %v", err)
	}
	wei, err := mobile.AmountToWei(chainID, token, envOr("HINKAL_E2E_AMOUNT", "0.1"))
	if err != nil {
		t.Fatalf("AmountToWei: %v", err)
	}

	raw, err := h.DepositAndWithdraw(chainID, token, `["`+wei+`"]`, `["`+addr+`"]`, 0, "", false)
	if err != nil {
		t.Fatalf("DepositAndWithdraw: %v", err)
	}
	var res struct {
		DepositTxHash string `json:"depositTxHash"`
		ScheduleID    string `json:"scheduleId"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("bad DepositAndWithdraw JSON %q: %v", raw, err)
	}
	if res.ScheduleID == "" {
		t.Fatal("DepositAndWithdraw returned no scheduleId")
	}

	status, err := h.CheckSendTransactionStatus(res.ScheduleID)
	if err != nil {
		t.Fatalf("CheckSendTransactionStatus(%s): %v", res.ScheduleID, err)
	}
	var parsed struct {
		ScheduleID   string `json:"scheduleId"`
		ChainID      int    `json:"chainId"`
		Transactions []struct {
			Status string `json:"status"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal([]byte(status), &parsed); err != nil {
		t.Fatalf("bad status JSON %q: %v", status, err)
	}
	if parsed.ScheduleID != res.ScheduleID {
		t.Errorf("status scheduleId = %s, want %s", parsed.ScheduleID, res.ScheduleID)
	}
	if len(parsed.Transactions) == 0 {
		t.Error("status carries no transaction legs")
	}
}
