package tests

import (
	"encoding/json"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/mobile"
)

const mobileFixedSignature = "0x" +
	"1111111111111111111111111111111111111111111111111111111111111111" +
	"2222222222222222222222222222222222222222222222222222222222222222" +
	"1b"

const mobileExpectedShieldedPublicKey = "0x1134dd7dad39fd446190a61f0c3005ef3afe3ee9bc8a941f0c01a1d3b7161260"

type fakeMobileHost struct {
	sig   string
	addr  string
	chain int
}

func (f fakeMobileHost) Address() (string, error)              { return f.addr, nil }
func (f fakeMobileHost) ChainID() int64                        { return int64(f.chain) }
func (f fakeMobileHost) PersonalSign(_ string) (string, error) { return f.sig, nil }
func (f fakeMobileHost) SwitchChain(_ int64) error             { return nil }
func (f fakeMobileHost) SendTransaction(_, _, _ string, _ int64) (string, error) {
	return "", nil
}

func TestMobileConnectDerivesShieldedKey(t *testing.T) {
	c := mobile.NewClient()
	out, err := c.Connect(fakeMobileHost{
		sig:   mobileFixedSignature,
		addr:  "0x1111111111111111111111111111111111111111",
		chain: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		ShieldedPublicKey string `json:"shieldedPublicKey"`
		EthAddress        string `json:"ethAddress"`
		RecipientInfo     string `json:"recipientInfo"`
		ChainID           int    `json:"chainId"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("bad Connect JSON %q: %v", out, err)
	}
	if res.ShieldedPublicKey != mobileExpectedShieldedPublicKey {
		t.Fatalf("shieldedPublicKey = %s, want %s", res.ShieldedPublicKey, mobileExpectedShieldedPublicKey)
	}
	if res.ChainID != 1 {
		t.Errorf("chainId = %d, want 1", res.ChainID)
	}
	if res.EthAddress == "" || res.RecipientInfo == "" {
		t.Errorf("empty ethAddress (%q) or recipientInfo (%q)", res.EthAddress, res.RecipientInfo)
	}
	t.Logf("Connect() = %s", out)
}

func TestMobileSetDNSServers(t *testing.T) {
	oldResolver := net.DefaultResolver
	defer func() { net.DefaultResolver = oldResolver }()

	cases := []struct {
		in   string
		want string
	}{
		{"", "8.8.8.8:53,1.1.1.1:53"},
		{"192.168.1.1", "192.168.1.1:53"},
		{"192.168.1.1:5353", "192.168.1.1:5353"},
		{"2001:4860:4860::8888", "[2001:4860:4860::8888]:53"},
		{"[2001:4860:4860::8888]:53", "[2001:4860:4860::8888]:53"},
		{"10.0.0.1, 10.0.0.2", "10.0.0.1:53,10.0.0.2:53"},
		{"not-an-ip,junk", "8.8.8.8:53,1.1.1.1:53"},
	}
	for _, c := range cases {
		if got := mobile.SetDNSServers(c.in); got != c.want {
			t.Errorf("SetDNSServers(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMobileChainHelpers(t *testing.T) {
	if !mobile.IsTronChain(mobile.TronChainID()) {
		t.Errorf("IsTronChain(TronChainID()) = false")
	}
	if !mobile.IsSolanaChain(mobile.SolanaChainID()) {
		t.Errorf("IsSolanaChain(SolanaChainID()) = false")
	}
	if mobile.IsTronChain(1) || mobile.IsSolanaChain(1) {
		t.Errorf("chain 1 flagged as tron/solana")
	}
	if mobile.SolanaNativeTokenAddress() == "" {
		t.Error("empty solana native token address")
	}
	all, err := mobile.AllSupportedChainsJSON()
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	if err := json.Unmarshal([]byte(all), &ids); err != nil {
		t.Fatalf("bad AllSupportedChainsJSON %q: %v", all, err)
	}
	var tron, sol bool
	for _, id := range ids {
		tron = tron || mobile.IsTronChain(int64(id))
		sol = sol || mobile.IsSolanaChain(int64(id))
	}
	if !tron || !sol {
		t.Errorf("AllSupportedChainsJSON() = %s, missing tron (%v) or solana (%v)", all, tron, sol)
	}
}

func seedWordsJSON(t *testing.T, phrase string) string {
	t.Helper()
	words, err := json.Marshal(strings.Fields(phrase))
	if err != nil {
		t.Fatal(err)
	}
	return string(words)
}

func TestMobileInitUserKeysFromSeedPhrasesDeterministic(t *testing.T) {
	a := mobile.NewClient().Hinkal()
	b := mobile.NewClient().Hinkal()
	seed := seedWordsJSON(t, "alpha bravo charlie delta echo foxtrot golf hotel india juliett kilo lima")
	if err := a.InitUserKeysFromSeedPhrases(seed); err != nil {
		t.Fatal(err)
	}
	if err := b.InitUserKeysFromSeedPhrases(seed); err != nil {
		t.Fatal(err)
	}
	pa, err := a.GetShieldedPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	pb, err := b.GetShieldedPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if pa == "" || pa != pb {
		t.Fatalf("shielded keys differ or empty: %q vs %q", pa, pb)
	}
	if err := a.InitUserKeysFromSeedPhrases("not json"); err == nil {
		t.Error("malformed seed phrase JSON accepted")
	}
}

func TestMobileInitUserKeysWithSignatureMatchesReference(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	h.InitUserKeysWithSignature(mobileFixedSignature)
	pub, err := h.GetShieldedPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if pub != mobileExpectedShieldedPublicKey {
		t.Fatalf("shieldedPublicKey = %s, want %s", pub, mobileExpectedShieldedPublicKey)
	}
	empty := mobile.NewClient().Hinkal()
	empty.InitUserKeysWithSignature("")
	if _, err := empty.GetShieldedPublicKey(); err == nil {
		t.Error("empty signature produced usable keys")
	}
}

func TestMobileConnectChainValidation(t *testing.T) {
	c := mobile.NewClient()
	pk := "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	if _, err := c.ConnectWithPrivateKey(pk, mobile.SolanaChainID()); err == nil {
		t.Error("EVM connect accepted a Solana chain id")
	}
	if _, err := c.ConnectWithTronPrivateKey(pk, 1); err == nil {
		t.Error("Tron connect accepted an EVM chain id")
	}
	if _, err := c.ConnectWithSolanaPrivateKey("3q2+7w==", 1); err == nil {
		t.Error("Solana connect accepted an EVM chain id")
	}
	if _, err := c.Connect(nil); err == nil {
		t.Error("Connect accepted a nil host")
	}
	if _, err := c.ConnectTron(nil, 0); err == nil {
		t.Error("ConnectTron accepted a nil host")
	}
	if _, err := c.ConnectSolana(nil, 0); err == nil {
		t.Error("ConnectSolana accepted a nil host")
	}
}

func TestMobileAmountValidation(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	for _, amounts := range []string{`["abc"]`, `[""]`, `["1.5"]`, `[1]`, "not json"} {
		if _, err := h.Deposit(1, `["0x0"]`, amounts, false, false); err == nil {
			t.Errorf("Deposit accepted amounts %q", amounts)
		}
		if _, err := h.Withdraw(1, `["0x0"]`, amounts, "0x1", false, "", ""); err == nil {
			t.Errorf("Withdraw accepted amounts %q", amounts)
		}
		if _, err := h.Transfer(1, `["0x0"]`, amounts, "0x1", "", ""); err == nil {
			t.Errorf("Transfer accepted amounts %q", amounts)
		}
		if _, err := h.Swap(1, `["0x0"]`, amounts, "Lifi", "", "", ""); err == nil {
			t.Errorf("Swap accepted amounts %q", amounts)
		}
	}
	if _, err := h.Deposit(1, `["0x0"]`, `["1","2"]`, false, false); err == nil {
		t.Error("Deposit accepted 1 token against 2 amounts")
	}
	if _, err := h.WaitForTransaction(1, "0xabc", -1, 0); err == nil {
		t.Error("WaitForTransaction accepted negative confirmations")
	}
}

func TestMobileSupportedChainsJSON(t *testing.T) {
	got, err := mobile.SupportedChainsJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "[") || got == "[]" || got == "null" {
		t.Fatalf("SupportedChainsJSON() = %q, want a non-empty JSON array", got)
	}
	for _, id := range []string{"1", "137", "42161", "10", "8453"} {
		if !strings.Contains(got, id) {
			t.Errorf("SupportedChainsJSON() = %q, missing chain id %s", got, id)
		}
	}
	t.Logf("SupportedChainsJSON() = %s", got)
}

func feeStructureJSON(feeToken, flatFee, variableRate string) string {
	return `{"feeToken":"` + feeToken + `","flatFee":"` + flatFee + `","variableRate":"` + variableRate + `"}`
}

func TestMobileCalculateTotalFee(t *testing.T) {
	c := mobile.NewClient()
	got, err := c.CalculateTotalFee("1000000", feeStructureJSON("0x0", "500", "0"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "500" {
		t.Errorf("CalculateTotalFee(flat only) = %s, want 500", got)
	}
	got, err = c.CalculateTotalFee("1000000", feeStructureJSON("0x0", "500", "200"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "20500" {
		t.Errorf("CalculateTotalFee(flat+variable) = %s, want 20500", got)
	}
	for _, tc := range []struct {
		amount, fee string
	}{
		{"1000", ""},
		{"1000", "not json"},
		{"abc", feeStructureJSON("0x0", "1", "0")},
		{"1000", feeStructureJSON("0x0", "", "0")},
	} {
		if _, err := c.CalculateTotalFee(tc.amount, tc.fee); err == nil {
			t.Errorf("CalculateTotalFee(%q, %q) accepted invalid input", tc.amount, tc.fee)
		}
	}
}

func TestMobileCalculateWithdrawalAmount(t *testing.T) {
	c := mobile.NewClient()
	got, err := c.CalculateWithdrawalAmount("-1000", feeStructureJSON("0x0", "100", "0"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "900" {
		t.Errorf("CalculateWithdrawalAmount = %s, want 900", got)
	}
	if _, err := c.CalculateWithdrawalAmount("-1000", ""); err == nil {
		t.Error("CalculateWithdrawalAmount accepted empty fee structure")
	}
	if _, err := c.CalculateWithdrawalAmount("xyz", feeStructureJSON("0x0", "1", "0")); err == nil {
		t.Error("CalculateWithdrawalAmount accepted bad amount")
	}
}

func TestMobileCalculateModifiedFeeStructureValidation(t *testing.T) {
	c := mobile.NewClient()
	for _, tc := range []struct {
		amount, fee string
	}{
		{"1000", ""},
		{"1000", "not json"},
		{"abc", feeStructureJSON("0x0", "1", "0")},
	} {
		if _, err := c.CalculateModifiedFeeStructure(1, "0x0", tc.amount, tc.fee); err == nil {
			t.Errorf("CalculateModifiedFeeStructure(%q, %q) accepted invalid input", tc.amount, tc.fee)
		}
	}
}

func TestMobileGetGasTokenSymbols(t *testing.T) {
	got, err := mobile.NewClient().GetGasTokenSymbols(0)
	if err != nil {
		t.Fatal(err)
	}
	var symbols []string
	if err := json.Unmarshal([]byte(got), &symbols); err != nil {
		t.Fatalf("bad GetGasTokenSymbols JSON %q: %v", got, err)
	}
	for _, want := range []string{"USDC", "USDT", "DAI"} {
		found := false
		for _, s := range symbols {
			found = found || s == want
		}
		if !found {
			t.Errorf("GetGasTokenSymbols(0) = %s, missing %s", got, want)
		}
	}
}

func TestMobileProoflessDepositWithPublicFeeRejectsSolana(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	structures := `[{"H0x":"1","H0y":"0","H1x":"0","H1y":"0","stealthAddress":"0"}]`
	_, err := h.ProoflessDepositWithPublicFee(int64(constants.CurrentSolanaChainID), "0x0", `["1000"]`, structures, "10", false, "", false)
	if err == nil {
		t.Fatal("ProoflessDepositWithPublicFee accepted a Solana-like chain where the fee would be dropped")
	}
	if !strings.Contains(err.Error(), "Solana") {
		t.Errorf("ProoflessDepositWithPublicFee error = %v, want it to mention Solana", err)
	}
}

func TestMobileStoreUtxoInEnclaveUnknownHandle(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	err := h.StoreUtxoInEnclave(1, "0xsender", "0xrecipient", "missing-handle", "sig")
	if err == nil {
		t.Fatal("StoreUtxoInEnclave accepted an unknown handle")
	}
	if !strings.Contains(err.Error(), "handle") {
		t.Errorf("StoreUtxoInEnclave error = %v, want it to mention the handle", err)
	}
}

func TestMobileLoginMessageModeConstants(t *testing.T) {
	if mobile.LoginMessageModeProtocol != "protocol" {
		t.Errorf("LoginMessageModeProtocol = %q, want %q", mobile.LoginMessageModeProtocol, "protocol")
	}
	if mobile.LoginMessageModePrivateTransfer != "privateTransfer" {
		t.Errorf("LoginMessageModePrivateTransfer = %q, want %q", mobile.LoginMessageModePrivateTransfer, "privateTransfer")
	}
}

func TestMobileEmporiumOp(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	out, err := h.EmporiumOp("0x1111111111111111111111111111111111111111", "0xdeadbeef", true, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "0x") {
		t.Errorf("EmporiumOp = %q, want 0x-prefixed hex", out)
	}
	if _, err := h.EmporiumOp("0x1111111111111111111111111111111111111111", "0x", false, "not-a-number"); err == nil {
		t.Error("EmporiumOp accepted a non-numeric value")
	}
	if _, err := h.EmporiumOp("0x1234", "0x", false, ""); err == nil {
		t.Error("EmporiumOp accepted an invalid endpoint address")
	}
}

func TestMobileBridgePrivateToPrivateRejectsBadRecipient(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	if _, err := h.BridgePrivateToPrivate(1, "0xToken", 8453, "0xToken", "1.0", "{not json", 0.5, ""); err == nil {
		t.Error("BridgePrivateToPrivate accepted invalid recipientJSON")
	}
	if _, err := h.BridgePrivateToPrivate(1, "0xToken", 8453, "0xToken", "1.0", `{"claimableNonce":"abc"}`, 0.5, ""); err == nil {
		t.Error("BridgePrivateToPrivate accepted a non-numeric claimableNonce")
	}
}

func TestMobileGetFeeStructureRejectsBadInput(t *testing.T) {
	h := mobile.NewClient().Hinkal()
	if _, err := h.GetFeeStructure(1, "", "{not json", "", "", "", ""); err == nil {
		t.Error("GetFeeStructure accepted invalid tokenAddrsJSON")
	}
	if _, err := h.GetFeeStructure(1, "", "", "", "[{bad", "", ""); err == nil {
		t.Error("GetFeeStructure accepted invalid callsJSON")
	}
}

func TestMobileChainIDsJSON(t *testing.T) {
	out, err := mobile.ChainIDsJSON()
	if err != nil {
		t.Fatal(err)
	}
	var ids map[string]int
	if err := json.Unmarshal([]byte(out), &ids); err != nil {
		t.Fatalf("bad ChainIDsJSON %q: %v", out, err)
	}
	if ids["EthMainnet"] != constants.ChainIDs.EthMainnet {
		t.Errorf("EthMainnet = %d, want %d", ids["EthMainnet"], constants.ChainIDs.EthMainnet)
	}
}

func TestMobileNetworkRegistryJSON(t *testing.T) {
	out, err := mobile.NetworkRegistryJSON()
	if err != nil {
		t.Fatal(err)
	}
	var registry map[string]struct {
		ChainID int
		Name    string
	}
	if err := json.Unmarshal([]byte(out), &registry); err != nil {
		t.Fatalf("bad NetworkRegistryJSON %q: %v", out, err)
	}
	if registry["1"].ChainID != 1 {
		t.Errorf("registry[1].ChainID = %d, want 1", registry["1"].ChainID)
	}

	single, err := mobile.NetworkJSON(1)
	if err != nil {
		t.Fatal(err)
	}
	var network struct{ ChainID int }
	if err := json.Unmarshal([]byte(single), &network); err != nil {
		t.Fatalf("bad NetworkJSON %q: %v", single, err)
	}
	if network.ChainID != 1 {
		t.Errorf("NetworkJSON(1).ChainID = %d, want 1", network.ChainID)
	}
	if _, err := mobile.NetworkJSON(999999); err == nil {
		t.Error("NetworkJSON accepted an unknown chain id")
	}
}

func TestMobileSignEddsaJSONDeterministic(t *testing.T) {
	k := mobile.NewUserKeys(mobileFixedSignature)
	first, err := k.SignEddsaJSON("12345")
	if err != nil {
		t.Fatal(err)
	}
	var sig struct {
		R8 []string `json:"r8"`
		S  string   `json:"s"`
	}
	if err := json.Unmarshal([]byte(first), &sig); err != nil {
		t.Fatalf("bad SignEddsaJSON %q: %v", first, err)
	}
	if len(sig.R8) != 2 || sig.R8[0] == "" || sig.R8[1] == "" || sig.S == "" {
		t.Fatalf("SignEddsaJSON = %q, want r8 pair and s", first)
	}
	second, err := mobile.NewUserKeys(mobileFixedSignature).SignEddsaJSON("12345")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("SignEddsaJSON not deterministic: %q vs %q", first, second)
	}
	if _, err := k.SignEddsaJSON("not-a-number"); err == nil {
		t.Error("SignEddsaJSON accepted a non-numeric message")
	}
}

func TestMobileWrapsWholePublicHinkalSurface(t *testing.T) {
	renamed := map[string]string{
		"GetUtxosFromEnclave": "FetchClaimableUtxos",
	}
	iface := reflect.TypeOf((*ihinkal.IHinkal)(nil)).Elem()
	wrapper := reflect.TypeOf(&mobile.Hinkal{})
	client := reflect.TypeOf(&mobile.Client{})
	for i := range iface.NumMethod() {
		name := iface.Method(i).Name
		if alt, ok := renamed[name]; ok {
			name = alt
		}
		if _, ok := wrapper.MethodByName(name); ok {
			continue
		}
		if _, ok := client.MethodByName(name); ok {
			continue
		}
		t.Errorf("IHinkal method %s has no mobile wrapper", iface.Method(i).Name)
	}
}
