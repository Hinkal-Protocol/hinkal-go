package transactions

import (
	"context"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func HinkalSolanaClaimUtxo(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	claimableUtxo *utxo.Utxo,
	feeStructureOverride *types.FeeStructure,
	claimableSignature string,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}
	if len(erc20Tokens) == 0 {
		return "", errClaimNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errClaimTooManyTokens
	}
	if claimableUtxo == nil || claimableUtxo.Amount == nil {
		return "", errClaimUtxoMissingKey
	}

	tokenAddress := erc20Tokens[0].Erc20TokenAddress
	utxoTokenAddress, err := claimableUtxo.GetTokenAddress(chainID)
	if err != nil {
		return "", err
	}
	if utxoTokenAddress != tokenAddress {
		return "", errClaimUtxoTokenMismatch
	}

	utxoSpecificUserKeys, resolvedNullifyingKey, err := claimUtxoUserKeys(claimableUtxo, claimableSignature)
	if err != nil {
		return "", err
	}
	if claimableUtxo.NullifyingKey != "" && claimableUtxo.NullifyingKey != resolvedNullifyingKey {
		return "", errClaimUtxoKeyMismatch
	}

	sourceUtxo, err := utxo.CreateFrom(claimableUtxo, types.UtxoParams{NullifyingKey: resolvedNullifyingKey})
	if err != nil {
		return "", err
	}
	paddingUtxo, err := claimPaddingUtxo(sourceUtxo, utxoSpecificUserKeys, resolvedNullifyingKey)
	if err != nil {
		return "", err
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, tokenAddress, []string{tokenAddress}, types.ExternalActionTransact, nil, nil, &api.SolanaGasEstimateParams{
			MintTo:         tokenAddress,
			NullifierCount: 1,
		})
		if err != nil {
			return "", err
		}
	}
	if feeStructure.FeeToken != tokenAddress {
		return "", errClaimFeeTokenMismatch
	}

	claimFeeStructure, recipientAmount, err := claimFeeParts(sourceUtxo.Amount, feeStructure)
	if err != nil {
		return "", err
	}
	totalRelayerFee := claimFeeStructure.FlatFee

	recipientInfo, err := hinkal.GetRecipientInfo()
	if err != nil {
		return "", err
	}
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	amountChange := new(big.Int).Neg(sourceUtxo.Amount)
	inputUtxos := []*utxo.Utxo{sourceUtxo, paddingUtxo}
	outputUtxos, err := pretransaction.OutputUtxoProcessing(
		utxoSpecificUserKeys,
		inputUtxos,
		amountChange,
		timeStamp,
		true,
		recipientInfo,
		recipientAmount,
	)
	if err != nil {
		return "", err
	}
	if len(outputUtxos) < 2 {
		return "", errSolanaTransferMissingOutput
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}

	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, resolvedNullifyingKey)
	if err != nil {
		return "", err
	}
	encryptedOutputBytes, encryptedOutputInts, err := solanaTransferEncryptedOutputs(outputUtxos)
	if err != nil {
		return "", err
	}

	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	outputUtxosArray := [][]*utxo.Utxo{outputUtxos}
	dimensions := types.DimDataType{
		TokenNumber:     1,
		NullifierAmount: len(inputUtxos),
		OutputAmount:    len(outputUtxos),
	}
	proof, err := snarkjs.ConstructSolanaZkProof(ctx, snarkjs.ConstructSolanaZkProofParams{
		GenerateProofRemotely: hinkal.GenerateProofRemotely(),
		MerkleTree:            hinkal.MerkleTree(chainID),
		UserKeys:              utxoSpecificUserKeys,
		MintAddresses:         []string{tokenAddress},
		InputUtxos:            inputUtxosArray,
		OutputUtxos:           outputUtxosArray,
		ExtraRandomization:    extraRandomization,
		RelayerFee:            totalRelayerFee,
		VariableRate:          big.NewInt(0),
		RecipientAddress:      constants.SolanaNativeAddress,
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
	})
	if err != nil {
		return "", err
	}

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:      chainID,
		RelayAddress: relay,
		FunctionName: "transfer",
		Args: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       totalRelayerFee.String(),
			Dimensions:       dimensions,
		},
		Accounts: api.SolanaTransactAccounts{
			Recipient: relay,
			Mint:      nonNativeMintString(tokenAddress),
		},
		CommitmentValidationData: proof.CommitmentValidationData,
	})
}
