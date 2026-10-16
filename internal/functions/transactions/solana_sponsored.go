package transactions

import (
	"context"
	"errors"
	"log"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func sponsoredSolanaSender(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	tempAccount types.TemporarySubAccount,
) (chainID int, senderAddress, signingKey, relayAddress string, err error) {
	chainID, err = pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
	if err != nil {
		return 0, "", "", "", err
	}
	senderAddress, err = privatewallet.AddressFromSubAccount(chainID, tempAccount)
	if err != nil {
		return 0, "", "", "", err
	}
	signingKey, err = privatewallet.PrivateKeyFromSubAccount(chainID, tempAccount)
	if err != nil {
		return 0, "", "", "", err
	}
	relayAddress, err = hinkal.GetRandomRelay(ctx, chainID, true)
	if err != nil {
		return 0, "", "", "", err
	}
	if relayAddress == "" {
		return 0, "", "", "", errors.New("transactions: no Solana relayer available")
	}
	return chainID, senderAddress, signingKey, relayAddress, nil
}

func emitSponsoredAdminData(
	ctx context.Context,
	action types.AdminTransactionType,
	chainID int,
	tokenAddress string,
	amount *big.Int,
	senderAddress string,
	variableRate *big.Int,
	txHash string,
) {
	adminData := pretransaction.ConstructAdminData(action, chainID, []string{tokenAddress}, []*big.Int{amount}, senderAddress, nil, variableRate)
	if adminData == nil {
		return
	}
	if err := api.EmitTxPublicData(ctx, adminData, txHash); err != nil {
		log.Printf("emit tx public data error: %v", err)
	}
}

func HinkalSolanaSponsoredShield(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	tempAccount types.TemporarySubAccount,
	landedAmount *big.Int,
	recipientInfo string,
	action types.AdminTransactionType,
	variableRate *big.Int,
) (string, *big.Int, error) {
	chainID, senderAddress, signingKey, relayAddress, err := sponsoredSolanaSender(ctx, hinkal, token, tempAccount)
	if err != nil {
		return "", nil, err
	}

	variableRateValue := ""
	if variableRate != nil {
		variableRateValue = variableRate.String()
	}

	txHash, feeWei, err := solanautils.SendSolanaSponsoredTx(ctx, signingKey, api.SolanaSponsoredBuildRequestBody{
		ChainID:       chainID,
		Kind:          types.SolanaSponsoredTxKindShield,
		TokenAddress:  token.Erc20TokenAddress,
		Amount:        landedAmount.String(),
		SenderAddress: senderAddress,
		RelayAddress:  relayAddress,
		RecipientInfo: recipientInfo,
		VariableRate:  variableRateValue,
	})
	if err != nil {
		return "", nil, err
	}

	shieldedAmount := new(big.Int).Sub(landedAmount, feeWei)
	emitSponsoredAdminData(ctx, action, chainID, token.Erc20TokenAddress, shieldedAmount, senderAddress, variableRate, txHash)

	return txHash, shieldedAmount, nil
}

func HinkalSolanaSponsoredProxySend(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	tempAccount types.TemporarySubAccount,
	landedAmount *big.Int,
	recipientAddress string,
	action types.AdminTransactionType,
) (string, error) {
	chainID, senderAddress, signingKey, relayAddress, err := sponsoredSolanaSender(ctx, hinkal, token, tempAccount)
	if err != nil {
		return "", err
	}

	txHash, _, err := solanautils.SendSolanaSponsoredTx(ctx, signingKey, api.SolanaSponsoredBuildRequestBody{
		ChainID:          chainID,
		Kind:             types.SolanaSponsoredTxKindSend,
		TokenAddress:     token.Erc20TokenAddress,
		Amount:           landedAmount.String(),
		SenderAddress:    senderAddress,
		RelayAddress:     relayAddress,
		RecipientAddress: recipientAddress,
	})
	if err != nil {
		return "", err
	}

	emitSponsoredAdminData(ctx, action, chainID, token.Erc20TokenAddress, landedAmount, senderAddress, nil, txHash)

	return txHash, nil
}
