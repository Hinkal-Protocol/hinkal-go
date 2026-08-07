package transactions

import (
	"context"
	"errors"
	"math/big"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const (
	twoDimensionalInputCount = 2
	sixDimensionalInputCount = 6
)

var (
	errTransactUserKeysNeedInputUtxos = errors.New("transactions: HinkalTransact: UserKeys requires explicit InputUtxos")
	errTransactEmptyInputUtxos        = errors.New("transactions: HinkalTransact: InputUtxos must contain at least one note per token")
)

func padInputUtxos(userKeys *cryptokeys.UserKeys, inputUtxos [][]*utxo.Utxo) ([][]*utxo.Utxo, error) {
	maxNotes := 0
	for _, notes := range inputUtxos {
		if len(notes) > maxNotes {
			maxNotes = len(notes)
		}
	}
	targetCount := sixDimensionalInputCount
	if maxNotes <= twoDimensionalInputCount {
		targetCount = twoDimensionalInputCount
	}

	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	padded := make([][]*utxo.Utxo, len(inputUtxos))
	for i, notes := range inputUtxos {
		if len(notes) == 0 {
			return nil, errTransactEmptyInputUtxos
		}
		reference := notes[0]
		capacity := targetCount
		if len(notes) > capacity {
			capacity = len(notes)
		}
		tokenNotes := make([]*utxo.Utxo, len(notes), capacity)
		copy(tokenNotes, notes)
		for len(tokenNotes) < targetCount {
			padding, err := utxo.NewUtxo(types.UtxoParams{
				Amount:            big.NewInt(0),
				Erc20TokenAddress: reference.Erc20TokenAddress,
				MintAddress:       reference.MintAddress,
				NullifyingKey:     shieldedPrivateKey,
				SpendingPublicKey: spendingPublicKey,
			})
			if err != nil {
				return nil, err
			}
			tokenNotes = append(tokenNotes, padding)
		}
		padded[i] = tokenNotes
	}
	return padded, nil
}

func buildProofLocally(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	params HinkalTransactParams,
	userKeys *cryptokeys.UserKeys,
) (TransactProof, error) {
	var inputUtxosArray [][]*utxo.Utxo
	var err error
	if params.InputUtxos != nil {
		inputUtxosArray, err = padInputUtxos(userKeys, params.InputUtxos)
	} else {
		inputUtxosArray, err = balance.AddPaddingToUtxos(
			ctx, hinkal, params.ChainID, params.Erc20Addresses, params.AmountChanges, 0, params.ForceEmptyUtxos, params.UseBlockedUtxos,
		)
	}
	if err != nil {
		return TransactProof{}, err
	}

	timeStamp := strconv.FormatInt(utils.GetCurrentTimeInSeconds(), 10)
	outputUtxosArray := make([][]*utxo.Utxo, len(params.Erc20Addresses))
	for i := range params.Erc20Addresses {
		recipientAmount := recipientAmountAt(params, i)
		outputUtxos, err := pretransaction.OutputUtxoProcessing(
			userKeys, inputUtxosArray[i], params.AmountChanges[i], timeStamp, true, params.RecipientAddress, recipientAmount,
		)
		if err != nil {
			return TransactProof{}, err
		}
		outputUtxosArray[i] = outputUtxos
	}

	externalActionMetadata := params.EmporiumOps
	if externalActionMetadata == nil {
		externalActionMetadata = params.ExternalActionMetadata
	}

	proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(params.ChainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               userKeys,
		ExternalActionID:       params.ExternalActionID,
		ExternalAddress:        params.ExternalAddress,
		ExternalActionMetadata: externalActionMetadata,
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           feeStructureOrZero(params.FeeStructure),
		Relay:                  params.Relay,
		ChainID:                params.ChainID,
		OnChainCreation:        params.OnChainCreation,
		OriginalSender:         params.OriginalSender,
		SubAccountPrivateKey:   params.SubAccountPrivateKey,
	})
	if err != nil {
		return TransactProof{}, err
	}

	return TransactProof{
		ZkCallData:               proof.ZkCallData,
		CircomData:               proof.CircomData,
		DimData:                  proof.DimData,
		CommitmentValidationData: proof.CommitmentValidationData,
	}, nil
}

func prepareEnclaveJob(
	ctx context.Context,
	params HinkalTransactParams,
	userKeys *cryptokeys.UserKeys,
) (string, types.EddsaSignature, error) {
	var inputCommitments []string
	for _, notes := range params.InputUtxos {
		for _, note := range notes {
			if note.Amount == nil || note.Amount.Sign() <= 0 {
				continue
			}
			commitment, err := note.GetCommitment()
			if err != nil {
				return "", types.EddsaSignature{}, err
			}
			inputCommitments = append(inputCommitments, commitment)
		}
	}

	var externalActionMetadata string
	if len(params.ExternalActionMetadata) > 0 {
		externalActionMetadata = params.ExternalActionMetadata[0]
	}

	prepared, err := enclave.PrepareTxEnclaveCall(ctx, userKeys, types.PrepareTxParams{
		ChainID:                params.ChainID,
		Erc20Addresses:         params.Erc20Addresses,
		AmountChanges:          params.AmountChanges,
		ExternalAddress:        params.ExternalAddress,
		OriginalSender:         params.OriginalSender,
		Relay:                  params.Relay,
		FeeStructure:           params.FeeStructure,
		ExternalActionID:       params.ExternalActionID,
		ExternalActionMetadata: externalActionMetadata,
		OnChainCreation:        params.OnChainCreation,
		RecipientAddress:       params.RecipientAddress,
		RecipientAmounts:       params.RecipientAmounts,
		InputCommitments:       inputCommitments,
		UseBlockedUtxos:        params.UseBlockedUtxos,
		ForceEmptyUtxos:        params.ForceEmptyUtxos,
		SkipLock:               params.SkipLock,
		MessageSeed:            params.MessageSeed,
	})
	if err != nil {
		return "", types.EddsaSignature{}, err
	}

	signedMessageHash, err := utils.ParseBigInt(prepared.SignedMessageHash)
	if err != nil {
		return "", types.EddsaSignature{}, err
	}
	sig, err := userKeys.SignEddsa(signedMessageHash)
	if err != nil {
		return "", types.EddsaSignature{}, err
	}
	return prepared.JobID, sig, nil
}

func feeStructureOrZero(fee *types.FeeStructure) types.FeeStructure {
	if fee == nil {
		return types.ZeroFeeStructure()
	}
	return *fee
}

func recipientAmountAt(params HinkalTransactParams, index int) *big.Int {
	if params.RecipientAddress == "" {
		return nil
	}
	if index < len(params.RecipientAmounts) && params.RecipientAmounts[index] != nil {
		return params.RecipientAmounts[index]
	}
	return big.NewInt(0)
}

func submitAndConfirm(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	params HinkalTransactParams,
	proof TransactProof,
) (TransactResult, error) {
	result, err := SubmitProof(ctx, hinkal, params, proof)
	if err != nil {
		return result, err
	}
	if params.OnTxConfirm != nil && params.Submit.Mode != SubmitModeProofOnly {
		if err := params.OnTxConfirm(proof.CircomData); err != nil {
			return result, err
		}
	}
	return result, nil
}

func HinkalTransact(ctx context.Context, hinkal ihinkal.HinkalInternal, params HinkalTransactParams) (TransactResult, error) {
	if params.UserKeys != nil && params.InputUtxos == nil {
		return TransactResult{}, errTransactUserKeysNeedInputUtxos
	}

	userKeys := params.UserKeys
	if userKeys == nil {
		userKeys = hinkal.GetUserKeys()
	}

	if hinkal.GenerateProofRemotely() && constants.IsEnclaveTxChain(params.ChainID) {
		jobID, sig, err := prepareEnclaveJob(ctx, params, userKeys)
		if err != nil {
			return TransactResult{}, err
		}

		if params.Submit.Mode == SubmitModeRelayer {
			relayer := params.Submit.Relayer
			txHash, err := enclave.FinalizeTxEnclaveCallRelay(ctx, jobID, sig, params.ChainID, enclave.FinalizeTxRelayExtra{
				AdminData:             relayer.AdminData,
				AuthorizationData:     relayer.AuthorizationData,
				WithUniswapWorkAround: &relayer.WithUniswapWorkAround,
			})
			return TransactResult{TxHash: txHash}, err
		}

		final, err := enclave.FinalizeTxEnclaveCall(ctx, jobID, sig)
		if err != nil {
			return TransactResult{}, err
		}
		circomData, err := snarkjs.DeserializeCircomData(final.CircomData)
		if err != nil {
			return TransactResult{}, err
		}
		var tronProofSignature *api.TronProofSignature
		if final.TronProofSignature != nil {
			sig := api.TronProofSignature(*final.TronProofSignature)
			tronProofSignature = &sig
		}
		return submitAndConfirm(ctx, hinkal, params, TransactProof{
			ZkCallData:               final.ZkCallData,
			CircomData:               circomData,
			DimData:                  final.DimData,
			CommitmentValidationData: final.CommitmentValidationData,
			TronProofSignature:       tronProofSignature,
		})
	}

	proof, err := buildProofLocally(ctx, hinkal, params, userKeys)
	if err != nil {
		return TransactResult{}, err
	}

	return submitAndConfirm(ctx, hinkal, params, proof)
}
