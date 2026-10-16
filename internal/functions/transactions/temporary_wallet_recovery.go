package transactions

import (
	"context"
	"errors"
	"math/big"

	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const claimableNonceBytes = 31

func generateClaimableNonce() (*big.Int, error) {
	return utils.RandomBigInt(claimableNonceBytes)
}

func createClaimableRecipientInfo(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	recipientEthAddress string,
) (string, error) {
	claimableNonce, err := generateClaimableNonce()
	if err != nil {
		return "", err
	}
	claimableSignature, err := hinkal.GetUserKeys().GetClaimableSignatureFromNonce(claimableNonce)
	if err != nil {
		return "", err
	}

	claimableKeys := cryptokeys.NewUserKeys(claimableSignature)
	recipientInfo, err := pretransaction.GetRecipientInfoFromUserKeys(claimableKeys)
	if err != nil {
		return "", err
	}

	senderAddress, err := hinkal.GetEthereumAddress(ctx)
	if err != nil {
		return "", err
	}

	claimableShieldedPrivateKey, err := claimableKeys.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	signature, err := hinkal.GetUserKeys().GetSignature()
	if err != nil {
		return "", err
	}
	if err := enclave.StoreClaimableKeyInEnclave(
		ctx,
		senderAddress,
		recipientEthAddress,
		claimableShieldedPrivateKey,
		chainID,
		claimableSignature,
		signature,
	); err != nil {
		return "", err
	}
	return recipientInfo, nil
}

func resolveRecoveryRecipientInfo(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	shieldEligible bool,
	enteredAddress string,
) (string, error) {
	if !shieldEligible {
		return "", nil
	}
	if enteredAddress == "" {
		return hinkal.GetRecipientInfo()
	}

	allocations, err := enclave.AllocateRecipientInfos(ctx, []string{enteredAddress})
	if err != nil {
		return "", err
	}
	resolvedRecipientInfo := ""
	if len(allocations) > 0 {
		resolvedRecipientInfo = allocations[0].RecipientInfo
	}

	if resolvedRecipientInfo != "" && pretransaction.IsValidPrivateAddress(resolvedRecipientInfo) {
		return resolvedRecipientInfo, nil
	}

	return createClaimableRecipientInfo(ctx, hinkal, chainID, enteredAddress)
}

func recoverSolanaBalance(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	amount *big.Int,
	subAccount types.TemporarySubAccount,
	shieldEligible bool,
	enteredAddress string,
) error {
	targetRecipientInfo, err := resolveRecoveryRecipientInfo(ctx, hinkal, chainID, shieldEligible, enteredAddress)
	if err != nil {
		return err
	}

	if targetRecipientInfo == "" {
		return errors.New("Target recipient info not found")
	}
	_, _, err = HinkalSolanaSponsoredShield(ctx, hinkal, token, subAccount, amount, targetRecipientInfo, "", nil)
	return err
}

func recoverEvmBalance(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	amount *big.Int,
	subAccount types.TemporarySubAccount,
	recipientAddress string,
	shieldEligible bool,
	enteredAddress string,
	withdrawCalls []types.CallInfo,
) error {
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		token.Erc20TokenAddress,
		[]string{token.Erc20TokenAddress},
		types.ExternalActionEmporium,
		withdrawCalls,
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	recoverAmount := new(big.Int).Sub(amount, fees.CalculateTotalFee(amount, feeStructure))
	if recoverAmount.Sign() <= 0 {
		return errInsufficientRecoveryFees
	}

	if err := hinkal.ResetMerkleTreesIfNecessary(ctx, chainID); err != nil {
		return err
	}

	if !shieldEligible {
		_, err := hinkalProxySend(
			ctx,
			hinkal,
			[]types.ERC20Token{token},
			[]*big.Int{recoverAmount},
			subAccount,
			recipientAddress,
			feeStructure.FeeToken,
			&feeStructure,
			"",
		)
		return err
	}

	if enteredAddress == "" {
		_, err := hinkalActionReceive(
			ctx,
			hinkal,
			[]types.ERC20Token{token},
			[]*big.Int{recoverAmount},
			feeStructure.FeeToken,
			&feeStructure,
			&subAccount,
			"",
		)
		return err
	}

	targetRecipientInfo, err := resolveRecoveryRecipientInfo(ctx, hinkal, chainID, shieldEligible, enteredAddress)
	if err != nil {
		return err
	}
	if targetRecipientInfo == "" {
		return errors.New("Target recipient info not found")
	}

	// hinkal.proxyToPrivate (hinkalProxyToPrivate.ts)
	walletAddress, _ := privatewallet.AddressFromSubAccount(chainID, subAccount)
	if walletAddress == "" {
		return errors.New("Public account address is not available")
	}
	deltaAmounts := pretransaction.ModifyVolatileTokenAmountChanges(ctx, chainID, []types.ERC20Token{token}, []*big.Int{recoverAmount}, walletAddress)
	modifiedFeeStructure := fees.CalculateModifiedFeeStructureForTokens(ctx, chainID, []types.ERC20Token{token}, deltaAmounts, feeStructure)
	_, err = HinkalProxyToPrivate(
		ctx,
		hinkal,
		chainID,
		token,
		deltaAmounts[0],
		subAccount,
		targetRecipientInfo,
		feeStructure.FeeToken,
		&modifiedFeeStructure,
		"",
	)
	return err
}
