package tests

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func ensure0x(s string) string {
	if strings.HasPrefix(s, "0x") {
		return s
	}
	return "0x" + s
}

func buildUtxoPlaintext(amount *big.Int, token, stealth string, ts *big.Int) []byte {
	var b strings.Builder
	b.WriteString(utils.ToBeHex(amount))
	b.WriteString(ensure0x(token))
	b.WriteString(ensure0x(stealth))
	b.WriteString(utils.ToBeHex(ts))
	b.WriteString(utils.ToBeHex(big.NewInt(7)))
	b.WriteString(utils.ToBeHex(big.NewInt(9)))
	b.WriteString("0x" + strings.Repeat("cc", 32))
	return []byte(b.String())
}

func sampleUtxoFields() (amount *big.Int, token, stealth string, ts *big.Int) {
	return big.NewInt(1_000_000),
		"0x" + strings.Repeat("11", 20),
		"0x" + strings.Repeat("2a", 31),
		big.NewInt(1_700_000_000)
}

func TestDecryptUtxo_SealedKeysRoundTrip(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	_, pk, err := cryptokeys.EncryptionKeyPair(spk)
	if err != nil {
		t.Fatal(err)
	}

	amount, token, stealth, ts := sampleUtxoFields()
	sealed, err := cryptokeys.EncryptSealedKeys(buildUtxoPlaintext(amount, token, stealth, ts), []*[32]byte{&pk})
	if err != nil {
		t.Fatal(err)
	}
	u, err := balance.DecryptUtxo(sealed, uk)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "token", u.Erc20TokenAddress, token)
	assertEqual(t, "stealth", u.StealthAddress, stealth)
	assertEqual(t, "timestamp", u.TimeStamp, ts.String())

	if c, err := u.GetCommitment(); err != nil || c == "" {
		t.Fatalf("commitment: %q err=%v", c, err)
	}
	if n, err := u.GetNullifier(); err != nil || n == "" {
		t.Fatalf("nullifier: %q err=%v", n, err)
	}
}

func TestDecryptUtxo_WrongKeyFails(t *testing.T) {
	owner := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	other := cryptokeys.NewUserKeys("0x" + strings.Repeat("ef", 65))

	spk, _ := owner.GetShieldedPrivateKey()
	_, pk, _ := cryptokeys.EncryptionKeyPair(spk)
	amount, token, stealth, ts := sampleUtxoFields()
	sealed, _ := cryptokeys.EncryptSealedKeys(buildUtxoPlaintext(amount, token, stealth, ts), []*[32]byte{&pk})

	if _, err := balance.DecryptUtxo(sealed, other); err == nil {
		t.Fatal("expected decryption to fail for a non-recipient key")
	}
}

func TestDecodeEvmUtxo_OnChainTuple(t *testing.T) {
	uk := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("03", 32))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(77_000)
	token := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	randomization := big.NewInt(98765)
	h0 := babyjub.NewPoint().Mul(randomization, babyjub.B8)
	h1 := h1FromH0(t, h0, spk)
	stealthAddress := big.NewInt(424242)
	encoded := buildAbiEncodedOnChainEvmUtxo(t, amount, token, h0.X, h0.Y, h1.X, h1.Y, stealthAddress, big.NewInt(1_700_000_456))

	u, err := balance.DecodeEvmUtxoHex(encoded, uk)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "token", u.Erc20TokenAddress, token.Hex())

	res, err := balance.Compute(
		[]*types.EncryptedOutputWithSign{{Value: encoded, IsPositive: false}},
		map[string]struct{}{},
		uk,
		constants.ChainIDs.EthMainnet,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Balances[token.Hex()]; got == nil || got.Cmp(amount) != 0 {
		t.Fatalf("balance = %v, want %s", got, amount)
	}

	wrong := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("02", 32))
	if _, err := balance.DecodeEvmUtxoHex(encoded, wrong); err == nil {
		t.Fatal("expected wrong key to fail ownership check")
	}
}

func TestDecodeSolanaOnChainUtxo(t *testing.T) {
	uk := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("04", 32))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(88_000)
	randomization := big.NewInt(45678)
	h0 := babyjub.NewPoint().Mul(randomization, babyjub.B8)
	h1 := h1FromH0(t, h0, spk)
	mintBytes := []byte{
		0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, 15,
		16, 17, 18, 19, 20, 21, 22, 23,
		24, 25, 26, 27, 28, 29, 30, 31,
	}
	mintPart1 := append(make([]byte, 16), mintBytes[:16]...)
	mintPart2 := append(make([]byte, 16), mintBytes[16:]...)
	encoded, err := utils.EncodeSolanaOnChainUtxo([][]byte{
		wordBytes(amount),
		mintPart1,
		mintPart2,
		wordBytes(h0.X),
		wordBytes(h0.Y),
		wordBytes(h1.X),
		wordBytes(h1.Y),
		wordBytes(h1.Y),
		wordBytes(big.NewInt(1_700_000_789)),
	})
	if err != nil {
		t.Fatal(err)
	}

	u, err := balance.DecodeSolanaOnChainUtxo(encoded, uk)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	mint := base58.Encode(mintBytes)
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "mint", u.MintAddress, mint)

	res, err := balance.Compute(
		[]*types.EncryptedOutputWithSign{{Value: encoded, IsPositive: false}},
		map[string]struct{}{},
		uk,
		constants.ChainIDs.SolanaMainnet,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Balances[mint]; got == nil || got.Cmp(amount) != 0 {
		t.Fatalf("balance = %v, want %s", got, amount)
	}
}

func TestEVMDecodeFromChain_Live(t *testing.T) {
	requireLive(t)
	rpcURL, err := constants.FetchRPCURL(constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Skip("ALCHEMY_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	raw, err := api.FetchSnapshots(ctx, constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}
	emitter, err := eventservice.NewEVMEmitter(constants.ChainIDs.EthMainnet, rpcURL, raw.Commitments.HinkalAddress, 0, nil)
	if err != nil {
		t.Fatalf("evm emitter: %v", err)
	}
	to, err := emitter.GetLastBlockNumberForEventRequest(ctx)
	if err != nil {
		t.Fatalf("last block: %v", err)
	}
	from := to - 20000
	events, err := emitter.GetEventsInRange(ctx, from, to)
	if err != nil {
		t.Fatalf("get events: %v", err)
	}

	newCommitments := 0
	for _, ev := range events {
		if ev.EventName != "NewCommitment" {
			continue
		}
		newCommitments++
		enc, err := ev.GetArg("encryptedOutput")
		if err != nil {
			t.Fatalf("getArg encryptedOutput: %v", err)
		}
		if !strings.HasPrefix(enc, "0x") {
			t.Fatalf("encryptedOutput not 0x-hex: %q", enc)
		}
		if _, err := hex.DecodeString(enc[2:]); err != nil {
			t.Fatalf("encryptedOutput not valid hex: %v", err)
		}
		c, err := ev.GetArg("commitment")
		if err != nil {
			t.Fatalf("getArg commitment: %v", err)
		}
		if _, err := utils.ParseBigInt(c); err != nil {
			t.Fatalf("commitment not parseable: %v", err)
		}
	}
	t.Logf("decoded %d NewCommitment events in blocks %d..%d", newCommitments, from, to)
	if newCommitments == 0 {
		t.Skip("no NewCommitment events in scanned range")
	}
}

func buildAbiEncodedOnChainEvmUtxo(
	t *testing.T,
	amount *big.Int,
	token common.Address,
	h0x, h0y, h1x, h1y, stealthAddress, ts *big.Int,
) string {
	t.Helper()
	stealthComponents := []abi.ArgumentMarshaling{
		{Name: "h0x", Type: "uint256"},
		{Name: "h0y", Type: "uint256"},
		{Name: "h1x", Type: "uint256"},
		{Name: "h1y", Type: "uint256"},
		{Name: "stealthAddress", Type: "uint256"},
	}
	utxoType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "amount", Type: "uint256"},
		{Name: "erc20Address", Type: "address"},
		{Name: "stealthAddressStructure", Type: "tuple", Components: stealthComponents},
		{Name: "timeStamp", Type: "uint256"},
	})
	if err != nil {
		t.Fatal(err)
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Type: utxoType}, {Type: bytesType}}

	type stealthTuple struct{ H0x, H0y, H1x, H1y, StealthAddress *big.Int }
	type utxoTuple struct {
		Amount                  *big.Int
		Erc20Address            common.Address
		StealthAddressStructure stealthTuple
		TimeStamp               *big.Int
	}
	packed, err := args.Pack(
		utxoTuple{
			Amount:       amount,
			Erc20Address: token,
			StealthAddressStructure: stealthTuple{
				H0x: h0x, H0y: h0y, H1x: h1x, H1y: h1y, StealthAddress: stealthAddress,
			},
			TimeStamp: ts,
		},
		[]byte{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(packed)
}

func h1FromH0(t *testing.T, h0 *babyjub.Point, privateKey string) *babyjub.Point {
	t.Helper()
	return babyjub.NewPoint().Mul(adjustedPrivateKey(t, privateKey), h0)
}

func adjustedPrivateKey(t *testing.T, privateKey string) *big.Int {
	t.Helper()
	n, err := utils.ParseBigInt(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return new(big.Int).Mod(n, crypto.FieldP)
}

func wordBytes(n *big.Int) []byte {
	word := make([]byte, 32)
	n.FillBytes(word)
	return word
}

// HINKAL_LIVE=1 HINKAL_SIGNATURE=0x... [HINKAL_CHAIN_ID=1] go test ./tests/... -run TestBalance_Live -v
func TestBalance_Live(t *testing.T) {
	requireLive(t)
	chainID := liveChainID(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	encOutputs, nullifierSet, snapshotBlock := loadChainState(t, ctx, chainID)

	uk := liveUserKeys(t, ctx, chainID)
	res, err := balance.Compute(encOutputs, nullifierSet, uk, chainID)
	if err != nil {
		t.Fatalf("compute balance: %v", err)
	}

	t.Logf("chain=%d snapshotBlock=%d outputs=%d nullifiers=%d ownedUtxos=%d",
		chainID, snapshotBlock, len(encOutputs), len(nullifierSet), len(res.Utxos))
	if len(res.Balances) == 0 {
		t.Logf("no balance for this signature on chain %d", chainID)
	}
	for token, amt := range res.Balances {
		t.Logf("balance %s = %s", token, amt.String())
	}
}
