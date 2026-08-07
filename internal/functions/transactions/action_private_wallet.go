package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types/bridging"
)

var errActionPrivateWalletSubAccountRequired = errors.New("transactions: subaccount is required for private wallet transactions")

func privateRecipientInfoAddress(info *bridging.PrivateRecipientInfo) string {
	if info == nil {
		return ""
	}
	return info.RecipientInfo
}

func privateRecipientAmounts(erc20Addresses []string, info *bridging.PrivateRecipientInfo) []*big.Int {
	if info == nil {
		return nil
	}
	amounts := make([]*big.Int, len(erc20Addresses))
	for i, address := range erc20Addresses {
		if strings.EqualFold(address, info.Token.Erc20TokenAddress) {
			amounts[i] = info.Amount
			continue
		}
		amounts[i] = big.NewInt(0)
	}
	return amounts
}

func actionPrivateWallet(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Tokens []types.ERC20Token,
	deltaChanges []*big.Int,
	onChainCreation []bool,
	ops []string,
	emporiumTokenChanges []bridging.TokenChange,
	subAccount *types.TemporarySubAccount,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
	relayOverride string,
	action types.AdminTransactionType,
	privateRecipientInfo *bridging.PrivateRecipientInfo,
) (string, error) {
	if privateRecipientInfo != nil && !pretransaction.IsValidPrivateAddress(privateRecipientInfo.RecipientInfo) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}

	if len(emporiumTokenChanges) > 0 && subAccount == nil {
		return "", errActionPrivateWalletSubAccountRequired
	}

	erc20Addresses := tokenAddresses(erc20Tokens)
	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return "", err
	}

	var walletAddress string
	if subAccount != nil {
		walletAddress = subAccount.EthAddress
	}

	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return "", err
	}
	if contractData.EmporiumAddress == "" {
		return "", errors.New("transactions: no emporium address provided")
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = normalizeFeeStructure(*feeStructureOverride)
	} else {
		calls := make([]types.CallInfo, len(ops))
		for i, op := range ops {
			call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, walletAddress, chainID)
			if err != nil {
				return "", err
			}
			calls[i] = call
		}
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, erc20Addresses, types.ExternalActionEmporium, calls, nil, nil)
		if err != nil {
			return "", err
		}
		feeStructure = normalizeFeeStructure(feeStructure)
	}

	if walletAddress != "" {
		var feeTokenChange *big.Int
		for _, change := range emporiumTokenChanges {
			if strings.EqualFold(change.Token.Erc20TokenAddress, feeStructure.FeeToken) {
				feeTokenChange = change.Amount
				break
			}
		}
		if err := mergeWithFeeStructureEmporium(ctx, chainID, walletAddress, &ops, &erc20Addresses, &deltaChanges, feeStructure, feeTokenChange); err != nil {
			return "", err
		}
	} else if err := pretransaction.MergeWithFeeStructure(ctx, chainID, &erc20Addresses, &deltaChanges, feeStructure); err != nil {
		return "", err
	}

	for len(onChainCreation) < len(erc20Addresses) {
		onChainCreation = append(onChainCreation, false)
	}

	useEnclave := hinkal.GenerateProofRemotely() && constants.IsEnclaveTxChain(chainID)

	relay := relayOverride
	if relay == "" {
		relay, err = relayerAddress(ctx, hinkal, chainID)
		if err != nil {
			return "", err
		}
	}

	var subAccountPrivateKey string
	if subAccount != nil {
		subAccountPrivateKey = subAccount.PrivateKey
	}

	var authorizationData *types.AuthorizationData
	if subAccountPrivateKey != "" {
		fetchClient, err := hinkal.GetFetchClient(chainID)
		if err != nil {
			return "", err
		}
		authorizationData, err = privatewallet.GetAuthorizationDataIfNeeded(ctx, fetchClient, chainID, subAccountPrivateKey)
		if err != nil {
			return "", err
		}
	}

	adminTokenAddresses := make([]string, len(emporiumTokenChanges))
	adminAmounts := make([]*big.Int, len(emporiumTokenChanges))
	for i, change := range emporiumTokenChanges {
		adminTokenAddresses[i] = change.Token.Erc20TokenAddress
		adminAmounts[i] = change.Amount
	}
	var swapPairTokens []types.ERC20Token
	if action == types.AdminPublicSwap {
		swapPairTokens = make([]types.ERC20Token, len(emporiumTokenChanges))
		for i, change := range emporiumTokenChanges {
			swapPairTokens[i] = change.Token
		}
	}
	adminData := pretransaction.ConstructAdminData(action, chainID, adminTokenAddresses, adminAmounts, ethereumAddress, swapPairTokens)

	messageSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", err
	}

	var externalActionMetadata []string
	if useEnclave {
		emporiumMessage, err := crypto.PoseidonBig(messageSeed)
		if err != nil {
			return "", err
		}
		subAccountSignerAddress := ""
		if subAccountPrivateKey != "" {
			subAccountSignerAddress, err = privatewallet.SignerAddressFromPrivateKey(chainID, subAccountPrivateKey)
			if err != nil {
				return "", err
			}
		}
		encoded, err := privatewallet.EncodeEmporiumMetadata(chainID, contractData.EmporiumAddress, subAccountPrivateKey, ops, emporiumMessage, subAccountSignerAddress)
		if err != nil {
			return "", err
		}
		externalActionMetadata = []string{encoded}
	}

	result, err := HinkalTransact(ctx, hinkal, HinkalTransactParams{
		ChainID:                chainID,
		Erc20Addresses:         erc20Addresses,
		AmountChanges:          deltaChanges,
		ExternalActionID:       types.ExternalActionEmporium,
		ExternalAddress:        contractData.EmporiumAddress,
		ExternalActionMetadata: externalActionMetadata,
		FeeStructure:           &feeStructure,
		Relay:                  relay,
		OnChainCreation:        onChainCreation,
		MessageSeed:            messageSeed,
		RecipientAddress:       privateRecipientInfoAddress(privateRecipientInfo),
		RecipientAmounts:       privateRecipientAmounts(erc20Addresses, privateRecipientInfo),
		SubAccountPrivateKey:   subAccountPrivateKey,
		EmporiumOps:            ops,
		Submit: NewRelayerSubmit(RelayerSubmit{
			AdminData:         adminData,
			AuthorizationData: authorizationData,
		}),
	})
	if err != nil {
		return "", err
	}
	return result.TxHash, nil
}
