package transactions

import (
	"context"
	"errors"
	"log"
	"math/big"

	"golang.org/x/sync/errgroup"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	privatewallet "github.com/Hinkal-Protocol/hinkal-go/internal/functions/private-wallet"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func getLifiBridgeRecipient(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	selectedChainID int,
	sourceToken types.ERC20Token,
	destToken types.ERC20Token,
	recipient string,
	amount string,
	tokenPriceDollar float64,
	quoteToTemporaryDestination bool,
	slippagePercentOverride *float64,
) (types.BridgeRecipient, error) {
	nonce, err := utils.RandomBigInt(6)
	if err != nil {
		return types.BridgeRecipient{}, err
	}
	wallets, err := privatewallet.DeriveBridgeTempWallets(hinkal.GetUserKeys(), selectedChainID, destToken.ChainID, nonce)
	if err != nil {
		return types.BridgeRecipient{}, err
	}
	walletPrivateKey := wallets.SourceSubAccount.PrivateKey
	sourceAddress, _ := privatewallet.AddressFromSubAccount(selectedChainID, wallets.SourceSubAccount)
	if walletPrivateKey == "" || sourceAddress == "" {
		return types.BridgeRecipient{}, errors.New("Failed to derive source temp wallet")
	}

	temporarySubAccount := types.TemporarySubAccount{
		Index:      int(nonce.Int64()),
		EthAddress: sourceAddress,
		PrivateKey: walletPrivateKey,
	}

	slippagePercent := getBridgingSlippagePercent(amount, tokenPriceDollar)
	if slippagePercentOverride != nil {
		slippagePercent = *slippagePercentOverride
	}
	destinationAddress, _ := privatewallet.AddressFromSubAccount(destToken.ChainID, wallets.DestinationSubAccount)
	toAddress := recipient
	if quoteToTemporaryDestination {
		toAddress = destinationAddress
	}
	if toAddress == "" {
		return types.BridgeRecipient{}, errors.New("Failed to derive destination temp wallet")
	}

	quote, err := web3.GetLifiPrice(
		ctx,
		sourceToken,
		destToken,
		amount,
		slippagePercent*0.01, // LIFI needs number between 0 and 1
		temporarySubAccount.EthAddress,
		toAddress,
	)
	if err != nil {
		return types.BridgeRecipient{}, err
	}

	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		return types.BridgeRecipient{}, err
	}
	return types.BridgeRecipient{
		RecipientAddress:    recipient,
		BridgeAmount:        bridgeAmount,
		Quote:               quote,
		TemporarySubAccount: temporarySubAccount,
	}, nil
}

func getLifiBridgeRecipients(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	sourceToken types.ERC20Token,
	destToken types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	slippagePercent *float64,
) ([]types.BridgeRecipient, error) {
	if len(recipientAmounts) != len(recipientAddresses) {
		return nil, errors.New("recipientAmounts and recipientAddresses length mismatch")
	}
	if len(recipientAmounts) == 0 {
		return nil, errors.New("No recipients to bridge")
	}

	var tokenPriceDollar float64
	if slippagePercent == nil {
		prices, err := api.GetTokenPrices(ctx, sourceToken.ChainID, []string{sourceToken.Erc20TokenAddress})
		if err != nil {
			log.Printf("getLifiBridgeRecipients: token price fetch failed: %v", err)
		} else if len(prices.Prices) > 0 {
			tokenPriceDollar = prices.Prices[0]
		}
	}

	isSameChainSwap := destToken.ChainID == sourceToken.ChainID
	bridgeRecipients := make([]types.BridgeRecipient, len(recipientAddresses))
	group, groupCtx := errgroup.WithContext(ctx)
	for i, recipient := range recipientAddresses {
		i, recipient := i, recipient
		group.Go(func() error {
			bridgeRecipient, err := getLifiBridgeRecipient(
				groupCtx,
				hinkal,
				sourceToken.ChainID,
				sourceToken,
				destToken,
				recipient,
				web3.GetAmountInToken(sourceToken, recipientAmounts[i]),
				tokenPriceDollar,
				false,
				slippagePercent,
			)
			if err != nil {
				return err
			}
			if !isSameChainSwap {
				destinationChainID := destToken.ChainID
				bridgeRecipient.DestinationChainID = &destinationChainID
			}
			bridgeRecipient.DestinationTokenAddress = destToken.Erc20TokenAddress
			bridgeRecipients[i] = bridgeRecipient
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return bridgeRecipients, nil
}
