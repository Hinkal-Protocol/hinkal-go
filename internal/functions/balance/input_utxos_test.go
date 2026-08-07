package balance_test

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/cache"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type fakeHinkal struct {
	ihinkal.HinkalInternal
	userKeys         *cryptokeys.UserKeys
	ethAddress       string
	encryptedOutputs []*types.EncryptedOutputWithSign
	nullifiers       map[string]struct{}
	cacheDevice      *cache.MemoryCacheDevice
	merkleUpdatesOff bool
}

func (f *fakeHinkal) GetUserKeys() *cryptokeys.UserKeys { return f.userKeys }
func (f *fakeHinkal) GetEthereumAddress(context.Context) (string, error) {
	return f.ethAddress, nil
}
func (f *fakeHinkal) EncryptedOutputs(int) []*types.EncryptedOutputWithSign {
	return f.encryptedOutputs
}
func (f *fakeHinkal) Nullifiers(int) map[string]struct{}        { return f.nullifiers }
func (f *fakeHinkal) CacheDevice() types.ICacheDevice           { return f.cacheDevice }
func (f *fakeHinkal) HinkalAddress(int) string                  { return "0xHinkalAddress" }
func (f *fakeHinkal) AllowParallelBalanceLocalDecryption() bool { return false }
func (f *fakeHinkal) AreMerkleTreeUpdatesDisabled() bool        { return f.merkleUpdatesOff }
func (f *fakeHinkal) GenerateProofRemotely() bool               { return false }

func newFakeHinkal(uk *cryptokeys.UserKeys, outputs []*types.EncryptedOutputWithSign) *fakeHinkal {
	return &fakeHinkal{
		userKeys:         uk,
		ethAddress:       "0xabc",
		encryptedOutputs: outputs,
		nullifiers:       map[string]struct{}{},
		cacheDevice:      cache.NewMemoryCacheDevice(),
	}
}

var defaultTestTokenAddress = "0x" + strings.Repeat("11", 20)

func sealedOutput(t *testing.T, recipientUk *cryptokeys.UserKeys, amount int64, isBlocked bool) *types.EncryptedOutputWithSign {
	t.Helper()
	return sealedOutputForToken(t, recipientUk, defaultTestTokenAddress, amount, isBlocked)
}

func sealedOutputForToken(t *testing.T, recipientUk *cryptokeys.UserKeys, tokenAddress string, amount int64, isBlocked bool) *types.EncryptedOutputWithSign {
	t.Helper()
	spk, err := recipientUk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	_, pk, err := cryptokeys.EncryptionKeyPair(spk)
	if err != nil {
		t.Fatal(err)
	}

	var plaintext strings.Builder
	plaintext.WriteString(utils.ToBeHex(big.NewInt(amount)))
	plaintext.WriteString(tokenAddress)
	plaintext.WriteString("0x" + strings.Repeat("2a", 31))
	plaintext.WriteString(utils.ToBeHex(big.NewInt(1_700_000_000)))
	plaintext.WriteString(utils.ToBeHex(big.NewInt(7)))
	plaintext.WriteString(utils.ToBeHex(big.NewInt(9)))
	plaintext.WriteString("0x" + strings.Repeat("cc", 32))

	sealed, err := cryptokeys.EncryptSealedKeys([]byte(plaintext.String()), []*[32]byte{&pk})
	if err != nil {
		t.Fatal(err)
	}
	return &types.EncryptedOutputWithSign{
		Value:      "0x" + hex.EncodeToString(sealed),
		IsPositive: true,
		IsBlocked:  isBlocked,
	}
}

func TestGetInputUtxoAndBalance_ColdCacheDecryptsLocallyAndFiltersToOwnedOutputs(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	other := cryptokeys.NewUserKeys("0x" + strings.Repeat("ef", 65))

	owned1 := sealedOutput(t, uk, 100, false)
	foreign := sealedOutput(t, other, 999, false) // sealed for someone else's key - must be skipped
	owned2 := sealedOutput(t, uk, 200, false)

	fake := newFakeHinkal(uk, []*types.EncryptedOutputWithSign{owned1, foreign, owned2})

	got, err := balance.GetInputUtxoAndBalance(context.Background(), balance.InputUtxoParams{
		Hinkal:     fake,
		ChainID:    constants.ChainIDs.EthMainnet,
		EthAddress: fake.ethAddress,
	})
	if err != nil {
		t.Fatalf("GetInputUtxoAndBalance: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (the foreign output must be silently skipped)", len(got))
	}
	if got[0].Amount.Int64() != 100 || got[1].Amount.Int64() != 200 {
		t.Fatalf("amounts = [%s %s], want [100 200]", got[0].Amount, got[1].Amount)
	}

	spk, err := uk.GetShieldedPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	cached, err := cache.GetHinkalCache(fake, constants.ChainIDs.EthMainnet, spk)
	if err != nil {
		t.Fatalf("GetHinkalCache: %v", err)
	}
	if cached.LastOutput != owned2.Value {
		t.Fatalf("cached LastOutput = %q, want the last scanned output's value", cached.LastOutput)
	}
	if len(cached.EncryptedOutputs) != 2 {
		t.Fatalf("cached EncryptedOutputs len = %d, want 2 (only the owned ones)", len(cached.EncryptedOutputs))
	}
}

func TestGetInputUtxoAndBalance_MerkleTreeUpdatesDisabledServesStaleCacheInstead(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	owned1 := sealedOutput(t, uk, 100, false) // already fetched and cached previously
	owned2 := sealedOutput(t, uk, 200, false) // a new output the chain produced since

	run := func(t *testing.T, merkleUpdatesOff bool) []int64 {
		t.Helper()
		fake := newFakeHinkal(uk, []*types.EncryptedOutputWithSign{owned1, owned2})
		fake.merkleUpdatesOff = merkleUpdatesOff

		spk, err := uk.GetShieldedPublicKey()
		if err != nil {
			t.Fatal(err)
		}
		if err := cache.SetHinkalCache(cache.HinkalCacheInterface{
			EncryptedOutputs: []*types.EncryptedOutputWithSign{owned1},
			LastOutput:       owned1.Value,
		}, fake, constants.ChainIDs.EthMainnet, spk); err != nil {
			t.Fatalf("SetHinkalCache: %v", err)
		}

		got, err := balance.GetInputUtxoAndBalance(context.Background(), balance.InputUtxoParams{
			Hinkal:     fake,
			ChainID:    constants.ChainIDs.EthMainnet,
			EthAddress: fake.ethAddress,
		})
		if err != nil {
			t.Fatalf("GetInputUtxoAndBalance: %v", err)
		}
		amounts := make([]int64, len(got))
		for i, u := range got {
			amounts[i] = u.Amount.Int64()
		}
		return amounts
	}

	t.Run("merkle updates enabled picks up the new on-chain output", func(t *testing.T) {
		got := run(t, false)
		if len(got) != 2 || got[0] != 100 || got[1] != 200 {
			t.Fatalf("amounts = %v, want [100 200]", got)
		}
	})

	t.Run("merkle updates disabled serves only what was already cached", func(t *testing.T) {
		got := run(t, true)
		if len(got) != 1 || got[0] != 100 {
			t.Fatalf("amounts = %v, want [100] - the new output must not be picked up while merkle updates are disabled", got)
		}
	})
}

func TestGetInputUtxoAndBalance_FiltersSpentUtxos(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	spent := sealedOutput(t, uk, 100, false)
	unspent := sealedOutput(t, uk, 200, false)

	decodedSpent, err := balance.DecryptUtxoHex(spent.Value, uk)
	if err != nil {
		t.Fatalf("decode spent output to find its nullifier: %v", err)
	}
	nullifier, err := decodedSpent.GetNullifier()
	if err != nil {
		t.Fatal(err)
	}

	fake := newFakeHinkal(uk, []*types.EncryptedOutputWithSign{spent, unspent})
	fake.nullifiers = map[string]struct{}{nullifier: {}}

	got, err := balance.GetInputUtxoAndBalance(context.Background(), balance.InputUtxoParams{
		Hinkal:     fake,
		ChainID:    constants.ChainIDs.EthMainnet,
		EthAddress: fake.ethAddress,
	})
	if err != nil {
		t.Fatalf("GetInputUtxoAndBalance: %v", err)
	}
	if len(got) != 1 || got[0].Amount.Int64() != 200 {
		t.Fatalf("got = %v, want only the unspent (200) utxo", got)
	}
}

func TestGetInputUtxoAndBalance_FiltersByBlockedFlag(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	normal := sealedOutput(t, uk, 100, false)
	blocked := sealedOutput(t, uk, 200, true)

	tests := []struct {
		name            string
		useBlockedUtxos bool
		wantAmount      int64
	}{
		{"default view returns the normal utxo", false, 100},
		{"blocked view returns the blocked utxo", true, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeHinkal(uk, []*types.EncryptedOutputWithSign{normal, blocked})
			got, err := balance.GetInputUtxoAndBalance(context.Background(), balance.InputUtxoParams{
				Hinkal:          fake,
				ChainID:         constants.ChainIDs.EthMainnet,
				EthAddress:      fake.ethAddress,
				UseBlockedUtxos: tt.useBlockedUtxos,
			})
			if err != nil {
				t.Fatalf("GetInputUtxoAndBalance: %v", err)
			}
			if len(got) != 1 || got[0].Amount.Int64() != tt.wantAmount {
				t.Fatalf("got = %v, want a single utxo with amount %d", got, tt.wantAmount)
			}
		})
	}
}
