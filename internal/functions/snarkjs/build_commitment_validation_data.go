package snarkjs

import (
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type BuildCommitmentValidationResult struct {
	TokenAddresses  []string
	InAmounts       [][]string
	InTimeStamps    [][]string
	InCommitments   [][]string
	InNullifiers    [][]string
	VerifierName    string
	CommitmentInput map[string]any
}

func BuildCommitmentValidationData(
	chainID int,
	userKeys *cryptokeys.UserKeys,
	erc20Addresses []string,
	inputUtxosArray [][]*utxo.Utxo,
) (*BuildCommitmentValidationResult, error) {
	if HasOnlyZeroAmounts(inputUtxosArray) {
		return nil, nil
	}
	if len(erc20Addresses) == 0 || len(inputUtxosArray) == 0 || len(inputUtxosArray[0]) == 0 {
		return nil, nil
	}

	nullifyingPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	defaultSpendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	tokenAddressesForCircuit := make([]string, len(erc20Addresses))
	for i, addr := range erc20Addresses {
		var n *big.Int
		if constants.IsSolanaLike(chainID) {
			formatted, err := solanautils.FormatMintAddress(addr)
			if err != nil {
				return nil, err
			}
			n, err = utils.ParseBigInt(formatted.CompressedAddress)
			if err != nil {
				return nil, err
			}
		} else {
			n, err = utils.ParseBigInt(addr)
			if err != nil {
				return nil, err
			}
		}
		tokenAddressesForCircuit[i] = n.String()
	}

	inAmounts := make([][]string, len(inputUtxosArray))
	inTimeStamps := make([][]string, len(inputUtxosArray))
	inH0Ax := make([][]string, len(inputUtxosArray))
	inH0Ay := make([][]string, len(inputUtxosArray))
	inCommitments := make([][]string, len(inputUtxosArray))

	for i, tokenUtxos := range inputUtxosArray {
		inAmounts[i] = make([]string, len(tokenUtxos))
		inTimeStamps[i] = make([]string, len(tokenUtxos))
		inH0Ax[i] = make([]string, len(tokenUtxos))
		inH0Ay[i] = make([]string, len(tokenUtxos))
		inCommitments[i] = make([]string, len(tokenUtxos))

		for j, u := range tokenUtxos {
			inAmounts[i][j] = u.Amount.String()
			inTimeStamps[i][j] = u.TimeStamp

			h0, err := GetUtxoCircuitH0Coords(u)
			if err != nil {
				return nil, err
			}
			inH0Ax[i][j] = h0[0].String()
			inH0Ay[i][j] = h0[1].String()

			if u.Amount.Sign() == 0 {
				inCommitments[i][j] = "0"
			} else {
				commitment, err := u.GetCommitment()
				if err != nil {
					return nil, err
				}
				commitmentBig, err := utils.ParseBigInt(commitment)
				if err != nil {
					return nil, err
				}
				inCommitments[i][j] = commitmentBig.String()
			}

			if err := validateUtxoStealth(u, nullifyingPrivateKey, defaultSpendingPublicKey, i, j); err != nil {
				return nil, err
			}
		}
	}

	inNullifiers, err := BuildInNullifiersByAmount(inputUtxosArray)
	if err != nil {
		return nil, err
	}

	commitmentInput := map[string]any{
		"nullifyingPrivateKey": nullifyingPrivateKey,
		"spendingPublicKey":    defaultSpendingPublicKey,
		"erc20TokenAddresses":  tokenAddressesForCircuit,
		"inAmounts":            inAmounts,
		"inH0Ax":               inH0Ax,
		"inH0Ay":               inH0Ay,
		"inTimeStamps":         inTimeStamps,
		"inCommitments":        inCommitments,
		"inNullifiers":         inNullifiers,
	}

	verifierName := fmt.Sprintf("commitmentCalculator%dx%d", len(erc20Addresses), len(inputUtxosArray[0]))

	return &BuildCommitmentValidationResult{
		TokenAddresses:  tokenAddressesForCircuit,
		InAmounts:       inAmounts,
		InTimeStamps:    inTimeStamps,
		InCommitments:   inCommitments,
		InNullifiers:    inNullifiers,
		VerifierName:    verifierName,
		CommitmentInput: commitmentInput,
	}, nil
}

func validateUtxoStealth(u *utxo.Utxo, nullifyingPrivateKey string, defaultSpendingPublicKey []*big.Int, tokenIdx, utxoIdx int) error {
	utxoStealth, err := u.GetStealthAddress()
	if err != nil {
		return err
	}
	spendingPublicKey := u.SpendingPublicKey
	if spendingPublicKey == nil {
		spendingPublicKey = defaultSpendingPublicKey
	}
	if u.H0 == nil {
		return fmt.Errorf("snarkjs: commitment validation missing H0 at tokenIdx=%d, utxoIdx=%d", tokenIdx, utxoIdx)
	}

	derived, err := cryptokeys.GetStealthAddress(*u.H0, nullifyingPrivateKey, spendingPublicKey)
	if err != nil {
		return err
	}

	utxoStealthBig, err := utils.ParseBigInt(utxoStealth)
	if err != nil {
		return err
	}
	derivedBig, err := utils.ParseBigInt(derived)
	if err != nil {
		return err
	}
	if utxoStealthBig.Cmp(derivedBig) != 0 {
		return fmt.Errorf("snarkjs: commitment validation stealth mismatch at tokenIdx=%d, utxoIdx=%d", tokenIdx, utxoIdx)
	}
	return nil
}

func BuildCommitmentValidationDataFromProof(
	build *BuildCommitmentValidationResult,
	proofResult *ZkProofResult,
) *types.CommitmentValidationDataType {
	if build == nil || proofResult == nil {
		return nil
	}
	return &types.CommitmentValidationDataType{
		TokenAddresses: build.TokenAddresses,
		InAmounts:      build.InAmounts,
		InTimeStamps:   build.InTimeStamps,
		InCommitments:  build.InCommitments,
		InNullifiers:   build.InNullifiers,
		Proof: types.CommitmentValidationProof{
			A: proofResult.ZkCallData.A,
			B: proofResult.ZkCallData.B,
			C: proofResult.ZkCallData.C,
		},
	}
}
