package tests

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/eventservice"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/snapshot"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// liveUserKeys resolves the account used by read-only balance tests with the same
// precedence newLiveEVMHinkal uses for transaction tests: HINKAL_SIGNATURE, then
// HINKAL_SEED_PHRASE, then a login signature derived from HINKAL_PRIVATE_KEY. Kept in
// one place so a balance check always looks at the same account a transaction would.
func liveUserKeys(t *testing.T, ctx context.Context, chainID int) *cryptokeys.UserKeys {
	t.Helper()
	if sig := os.Getenv("HINKAL_SIGNATURE"); sig != "" {
		return cryptokeys.NewUserKeys(sig)
	}
	h, _ := newLiveEVMHinkal(t, ctx, chainID)
	return h.GetUserKeys()
}

// newLiveBalanceHinkal is the newLiveEVMHinkal equivalent for tests that only ever
// read balances: it accepts HINKAL_SIGNATURE without requiring HINKAL_PRIVATE_KEY
// (no signer needed to decrypt your own shielded balance), and otherwise defers to
// newLiveEVMHinkal so seed-phrase/private-key resolution stays identical everywhere.
func newLiveBalanceHinkal(t *testing.T, ctx context.Context, chainID int) (*hinkal.Hinkal, string) {
	t.Helper()
	if sig := os.Getenv("HINKAL_SIGNATURE"); sig != "" {
		return newSignatureHinkal(t, ctx, chainID, sig)
	}
	return newLiveEVMHinkal(t, ctx, chainID)
}

func liveChainID(t *testing.T) int {
	t.Helper()
	chainID := constants.ChainIDs.EthMainnet
	if v := os.Getenv("HINKAL_CHAIN_ID"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("bad HINKAL_CHAIN_ID: %v", err)
		}
		chainID = parsed
	}
	return chainID
}

func newSignatureHinkal(t *testing.T, ctx context.Context, chainID int, signature string) (*hinkal.Hinkal, string) {
	t.Helper()
	h := hinkal.NewHinkal(nil)
	h.InitUserKeysWithSignature(signature)

	ethAddress, err := h.GetUserKeys().VerifyMessage(h.GetSigningMessage(types.LoginMessageModeProtocol))
	if err != nil {
		t.Fatalf("recover address from HINKAL_SIGNATURE: %v", err)
	}

	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}

	return h, ethAddress
}

func loadChainState(t *testing.T, ctx context.Context, chainID int) (encOutputs []*types.EncryptedOutputWithSign, nullifierSet map[string]struct{}, latestBlock uint64) {
	t.Helper()
	raw, err := api.FetchSnapshots(ctx, chainID)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}
	fetcher := snapshot.NewSnapshotFetcherService(chainID, raw.Commitments.HinkalAddress)

	if constants.IsSolanaLike(chainID) {
		emitter := eventservice.NewSolanaBlockchainEventEmitter(chainID, solanaRPC(), raw.Commitments.HinkalAddress, 0, false, nil, nil)
		commitments := snapshot.NewClientSolanaCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
		nullifiers := snapshot.NewClientSolanaNullifierSnapshotService(emitter, fetcher)
		if err := commitments.Svc.Init(ctx); err != nil {
			t.Fatalf("init commitments: %v", err)
		}
		if err := nullifiers.Svc.Init(ctx); err != nil {
			t.Fatalf("init nullifiers: %v", err)
		}
		if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
			t.Fatalf("gap-fill: %v", err)
		}
		return commitments.EncryptedOutputs(), nullifiers.Nullifiers(), raw.Commitments.LatestBlockNumber
	}

	rpcURL, rpcErr := constants.FetchRPCURL(chainID)
	if rpcErr != nil {
		t.Skipf("no RPC for chain %d: %v", chainID, rpcErr)
	}
	emitter, err := eventservice.NewEVMEmitter(chainID, rpcURL, raw.Commitments.HinkalAddress, 0, nil)
	if err != nil {
		t.Fatalf("evm emitter: %v", err)
	}
	commitments := snapshot.NewClientCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
	nullifiers := snapshot.NewClientNullifierSnapshotService(emitter, fetcher)
	if err := commitments.Svc.Init(ctx); err != nil {
		t.Fatalf("init commitments: %v", err)
	}
	if err := nullifiers.Svc.Init(ctx); err != nil {
		t.Fatalf("init nullifiers: %v", err)
	}
	if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
		t.Fatalf("gap-fill: %v", err)
	}
	return commitments.EncryptedOutputs(), nullifiers.Nullifiers(), raw.Commitments.LatestBlockNumber
}

func sumBalancesPerToken(utxos []*utxo.Utxo, chainID int) map[string]*big.Int {
	balances := map[string]*big.Int{}
	for _, u := range utxos {
		token := balanceKey(u, chainID)
		if balances[token] == nil {
			balances[token] = new(big.Int)
		}
		balances[token].Add(balances[token], u.Amount)
	}
	return balances
}

func balanceKey(u *utxo.Utxo, chainID int) string {
	if constants.IsSolanaLike(chainID) || constants.IsTronLike(chainID) {
		addr, _ := u.GetTokenAddress(chainID)
		return addr
	}
	return common.HexToAddress(u.Erc20TokenAddress).Hex()
}

// HINKAL_LIVE=1 HINKAL_SIGNATURE=0x... [HINKAL_CHAIN_ID=1] go test ./tests/... -run TestRemoteBalance_Live -v
func TestRemoteBalance_Live(t *testing.T) {
	requireLive(t)
	chainID := liveChainID(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	localHinkal, ethAddress := newLiveBalanceHinkal(t, ctx, chainID)
	localUtxos, err := balance.GetInputUtxoAndBalance(ctx, balance.InputUtxoParams{
		Hinkal:                localHinkal,
		ChainID:               chainID,
		EthAddress:            ethAddress,
		ResetCacheBefore:      true,
		AllowRemoteDecryption: false,
	})
	if err != nil {
		t.Fatalf("local decryption: %v", err)
	}

	remoteHinkal, remoteEthAddress := newLiveBalanceHinkal(t, ctx, chainID)
	remoteUtxos, err := balance.GetInputUtxoAndBalance(ctx, balance.InputUtxoParams{
		Hinkal:                remoteHinkal,
		ChainID:               chainID,
		EthAddress:            remoteEthAddress,
		ResetCacheBefore:      false,
		AllowRemoteDecryption: true,
	})
	if err != nil {
		t.Fatalf("remote decryption: %v", err)
	}

	localBalances := sumBalancesPerToken(localUtxos, chainID)
	remoteBalances := sumBalancesPerToken(remoteUtxos, chainID)

	t.Logf("chain=%d ethAddress=%s remoteEthAddress=%s localUtxos=%d remoteUtxos=%d", chainID, ethAddress, remoteEthAddress, len(localUtxos), len(remoteUtxos))
	for token, amt := range localBalances {
		t.Logf("local  %s = %s", token, amt.String())
	}
	for token, amt := range remoteBalances {
		t.Logf("remote %s = %s", token, amt.String())
	}

	if len(localBalances) != len(remoteBalances) {
		t.Errorf("token count mismatch: local=%d remote=%d", len(localBalances), len(remoteBalances))
	}
	for token, localAmt := range localBalances {
		remoteAmt, ok := remoteBalances[token]
		if !ok {
			t.Errorf("token %s present locally but missing remotely", token)
			continue
		}
		if localAmt.Cmp(remoteAmt) != 0 {
			t.Errorf("balance mismatch for %s: local=%s remote=%s", token, localAmt.String(), remoteAmt.String())
		}
	}
}

// HINKAL_LIVE=1 HINKAL_SIGNATURE=0x... [HINKAL_CHAIN_ID=1] go test ./tests/... -run TestHinkalGetTotalBalance_Live -v
func TestHinkalGetTotalBalance_Live(t *testing.T) {
	requireLive(t)
	chainID := liveChainID(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	h, ethAddress := newLiveBalanceHinkal(t, ctx, chainID)

	balances, err := h.GetTotalBalance(ctx, chainID, nil, ethAddress, false, false)
	if err != nil {
		t.Fatalf("getTotalBalance: %v", err)
	}

	t.Logf("chain=%d tokens=%d", chainID, len(balances))
	for _, b := range balances {
		t.Logf("balance %s = %s", b.Token.Erc20TokenAddress, b.Balance.String())
	}
}
