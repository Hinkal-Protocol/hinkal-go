package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/onchainutxos"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func claimRandomizationForMatch(u *utxo.Utxo) *big.Int {
	if u.H0 == nil {
		return nil
	}
	return (*u.H0)[0]
}

func matchClaimableDeposit(t *testing.T, deposited []*utxo.Utxo, planned *utxo.Utxo) *utxo.Utxo {
	t.Helper()
	plannedRandomization := claimRandomizationForMatch(planned)
	if plannedRandomization == nil {
		t.Fatalf("planned claimable UTXO missing randomization")
	}
	for _, candidate := range deposited {
		candidateRandomization := claimRandomizationForMatch(candidate)
		if candidate.Amount.Cmp(planned.Amount) == 0 && candidateRandomization != nil && candidateRandomization.Cmp(plannedRandomization) == 0 {
			return candidate
		}
	}
	t.Fatalf("could not match deposited claimable UTXO among %d candidates", len(deposited))
	return nil
}

func claimableUtxoFromEnclaveItem(item types.UtxoConstructorParamsWithSenderAddress) (*utxo.Utxo, error) {
	params := item.ResolvedUtxoParams()
	if params.NullifyingKey == "" && item.ClaimableSignature != "" {
		keys := cryptokeys.NewUserKeys(item.ClaimableSignature)
		nullifyingKey, err := keys.GetShieldedPrivateKey()
		if err != nil {
			return nil, err
		}
		params.NullifyingKey = nullifyingKey
	}
	return utxo.NewUtxo(params)
}

func effectiveVariableRate(feeStructure types.FeeStructure) *big.Int {
	if feeStructure.VariableRate == nil || feeStructure.VariableRate.Sign() <= 0 {
		return constants.HinkalPrivateSendVariableRate()
	}
	return feeStructure.VariableRate
}

func claimRecipientAmount(sourceAmount *big.Int, feeStructure types.FeeStructure) *big.Int {
	flatFee := feeStructure.FlatFee
	if flatFee == nil {
		flatFee = big.NewInt(0)
	}
	variableRate := effectiveVariableRate(feeStructure)
	transferable := new(big.Int).Sub(sourceAmount, flatFee)
	variableFee := new(big.Int).Div(new(big.Int).Mul(transferable, variableRate), big.NewInt(10000))
	return new(big.Int).Sub(transferable, variableFee)
}

func claimSourceAmountForRecipient(recipientAmount *big.Int, feeStructure types.FeeStructure) *big.Int {
	flatFee := feeStructure.FlatFee
	if flatFee == nil {
		flatFee = big.NewInt(0)
	}
	variableRate := effectiveVariableRate(feeStructure)
	denominator := new(big.Int).Sub(big.NewInt(10000), variableRate)
	transferable := new(big.Int).Div(new(big.Int).Mul(recipientAmount, big.NewInt(10000)), denominator)
	transferable.Add(transferable, big.NewInt(1))
	return new(big.Int).Add(flatFee, transferable)
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestClaimUtxoWithEnclave_Live -v
func TestClaimUtxoWithEnclave_Live(t *testing.T) {
	requireLive(t)
	chainID, token := evmTestChainAndToken(t)
	recipientTarget := big.NewInt(10_000)

	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)

	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, token, []string{token}, types.ExternalActionTransact, nil, nil, nil)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	claimableAmount := claimSourceAmountForRecipient(recipientTarget, feeStructure)

	claimableSignature, err := h.UserKeys.GetClaimableSignatureFromNonce(big.NewInt(time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("claimable signature: %v", err)
	}
	claimableKeys := cryptokeys.NewUserKeys(claimableSignature)
	shieldedPrivateKey, err := claimableKeys.GetShieldedPrivateKey()
	if err != nil {
		t.Fatalf("claimable shielded key: %v", err)
	}
	spendingKeyPair, err := claimableKeys.GetSpendingKeyPair()
	if err != nil {
		t.Fatalf("claimable spending key: %v", err)
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	// New-style UTXOs no longer carry a randomization, so the claimable deposit randomization is
	// generated explicitly (mirrors buildClaimableDepositPlan in @hinkal/common). The planned UTXO's
	// H0 is derived from it so it matches the on-chain deposited UTXO.
	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		t.Fatalf("random seed: %v", err)
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
	if err != nil {
		t.Fatalf("extra randomization: %v", err)
	}

	plannedUtxo, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            claimableAmount,
		Erc20TokenAddress: token,
		NullifyingKey:     shieldedPrivateKey,
		SpendingPublicKey: spendingPublicKey,
		Randomization:     extraRandomization,
	})
	if err != nil {
		t.Fatalf("planned UTXO: %v", err)
	}
	if plannedUtxo.H0 == nil {
		t.Fatalf("planned UTXO missing H0")
	}
	structure, err := snarkjs.CalcStealthAddressStructureFromH0(*plannedUtxo.H0, shieldedPrivateKey, spendingPublicKey)
	if err != nil {
		t.Fatalf("stealth structure: %v", err)
	}
	claimableEncPair, err := cryptokeys.GetEncryptionKeyPair(shieldedPrivateKey)
	if err != nil {
		t.Fatalf("claimable encryption key: %v", err)
	}

	_, depositTxHash, err := h.ProoflessDeposit(ctx, chainID, []string{token}, []*big.Int{claimableAmount}, []types.StealthAddressStructure{structure}, []string{claimableEncPair.PublicKey}, false, types.ZeroProoflessFeeStructure(), "", false)
	if err != nil {
		t.Fatalf("proofless deposit: %v", err)
	}
	t.Logf("claimable proofless deposit tx: %s (amount=%s)", depositTxHash, claimableAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}

	fetchClient, err := h.GetFetchClient(chainID)
	if err != nil {
		t.Fatalf("fetch client: %v", err)
	}
	receipt, err := web3.FetchTransactionReceiptWithRetry(ctx, fetchClient, depositTxHash)
	if err != nil {
		t.Fatalf("deposit receipt: %v", err)
	}
	depositedUtxos, err := onchainutxos.DecodeFromReceipt(receipt, claimableKeys, chainID, token)
	if err != nil {
		t.Fatalf("decode deposited claimable UTXOs: %v", err)
	}
	depositedUtxo := matchClaimableDeposit(t, depositedUtxos, plannedUtxo)
	utxoToStore, err := utxo.CreateFrom(depositedUtxo, types.UtxoParams{
		NullifyingKey:     shieldedPrivateKey,
		SpendingPublicKey: spendingPublicKey,
	})
	if err != nil {
		t.Fatalf("stored UTXO: %v", err)
	}
	storedCommitment, err := utxoToStore.GetCommitment()
	if err != nil {
		t.Fatalf("stored commitment: %v", err)
	}

	if err := h.StoreUtxoInEnclave(ctx, ethAddress, ethAddress, utxoToStore, chainID, claimableSignature); err != nil {
		t.Fatalf("store UTXO in enclave: %v", err)
	}
	signature, err := h.SignHinkalMessage(ctx, types.LoginMessageModeProtocol)
	if err != nil {
		t.Fatalf("enclave auth signature: %v", err)
	}
	fetchedItems, err := h.GetUtxosFromEnclave(ctx, ethAddress, signature, chainID, false, "")
	if err != nil {
		t.Fatalf("fetch UTXOs from enclave: %v", err)
	}

	var fetchedUtxo *utxo.Utxo
	for _, item := range fetchedItems {
		candidate, err := claimableUtxoFromEnclaveItem(item)
		if err != nil {
			t.Fatalf("map enclave item: %v", err)
		}
		commitment, err := candidate.GetCommitment()
		if err != nil {
			t.Fatalf("candidate commitment: %v", err)
		}
		if commitment == storedCommitment {
			fetchedUtxo = candidate
			break
		}
	}
	if fetchedUtxo == nil {
		t.Fatalf("stored claimable UTXO was not returned by enclave; fetched=%d", len(fetchedItems))
	}

	privateBefore := privateBalanceForToken(t, ctx, h, chainID, ethAddress, token)
	claimTxHash, err := h.ClaimUtxo(ctx, chainID, token, fetchedUtxo, &feeStructure, claimableSignature)
	if err != nil {
		t.Fatalf("claim UTXO: %v", err)
	}
	t.Logf("claim tx: %s", claimTxHash)
	if _, err := h.WaitForTransaction(ctx, chainID, claimTxHash, 1); err != nil {
		t.Fatalf("wait for claim tx: %v", err)
	}
	expected := claimRecipientAmount(claimableAmount, feeStructure)
	privateAfter := waitForPrivateBalanceDelta(t, ctx, h, chainID, ethAddress, token, privateBefore, expected)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	t.Logf("private USDC claim: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
