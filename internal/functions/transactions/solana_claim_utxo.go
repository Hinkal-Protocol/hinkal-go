package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errClaimInvalidRecipientInfo = errors.New("invalid recipient info")

func hasClaimRecipientParts(recipientInfo string) bool {
	parts := strings.Split(recipientInfo, ",")
	if len(parts) < 6 {
		return false
	}
	stealthAddress, h00, h01, encryptionKey := parts[0], parts[1], parts[2], parts[5]
	return stealthAddress != "" && h00 != "" && h01 != "" && encryptionKey != ""
}

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

	if err := ensureSolanaDeployData(chainID); err != nil {
		return "", err
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
	if !hasClaimRecipientParts(recipientInfo) {
		return "", errClaimInvalidRecipientInfo
	}
	amountChange := new(big.Int).Neg(sourceUtxo.Amount)
	inputUtxos := []*utxo.Utxo{sourceUtxo, paddingUtxo}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}

	result, err := SolanaTransact(ctx, hinkal, HinkalSolanaTransactParams{
		ChainID:       chainID,
		MintAddresses: []string{tokenAddress},
		AmountChanges: []*big.Int{amountChange},
		RelayAddress:  relay,
		Recipient:     constants.SolanaNativeAddress,
		Signer:        relay,
		FunctionName:  "transfer",
		Accounts: api.SolanaTransactAccounts{
			Recipient: relay,
			Mint:      nonNativeMintString(tokenAddress),
		},
		RelayerFee:       totalRelayerFee,
		VariableRate:     big.NewInt(0),
		RecipientAddress: recipientInfo,
		RecipientAmounts: []*big.Int{recipientAmount},
		InputUtxos:       [][]*utxo.Utxo{inputUtxos},
		UserKeys:         utxoSpecificUserKeys,
		Submit:           SolanaTransactSubmit{Mode: SolanaSubmitModeRelayer},
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
