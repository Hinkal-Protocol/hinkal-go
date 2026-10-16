package transactions

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func isNearIntentsRefundBalance(balance types.StuckBridgeBalance) bool {
	return balance.AssetID != ""
}

func GetStuckBridgeBalances(ctx context.Context, hinkal ihinkal.HinkalInternal, chainIDs []int) []types.StuckBridgeBalance {
	temporaryWalletChainIDs := chainIDs
	if chainIDs == nil {
		temporaryWalletChainIDs = constants.FrontEndSupportedChains()
	}

	var temporaryWalletBalances []types.TemporaryWalletBalance
	var nearIntentsRefundBalances []types.NearIntentsRefundBalance
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		temporaryWalletBalances = getTemporaryWalletBalances(ctx, hinkal, temporaryWalletChainIDs)
	}()
	go func() {
		defer wg.Done()
		if chainIDs == nil || slices.ContainsFunc(chainIDs, constants.UsesNearIntentsBridge) {
			nearIntentsRefundBalances = getNearIntentsRefundBalances(ctx, hinkal)
		}
	}()
	wg.Wait()

	stuckBridgeBalances := make([]types.StuckBridgeBalance, 0, len(temporaryWalletBalances)+len(nearIntentsRefundBalances))
	for _, balance := range temporaryWalletBalances {
		if !isRecoverableBalance(balance.TokenBalanceWithUsdValue) {
			continue
		}
		stuckBridgeBalances = append(stuckBridgeBalances, types.StuckBridgeBalance{
			TokenBalanceWithUsdValue: balance.TokenBalanceWithUsdValue,
			Nonce:                    balance.Nonce,
			EthAddress:               balance.EthAddress,
			PrivateKey:               balance.PrivateKey,
			RecoveryDestination:      balance.RecoveryDestination,
		})
	}
	for _, balance := range nearIntentsRefundBalances {
		if chainIDs != nil && !slices.Contains(chainIDs, balance.Token.ChainID) {
			continue
		}
		stuckBridgeBalances = append(stuckBridgeBalances, types.StuckBridgeBalance{
			TokenBalanceWithUsdValue: balance.TokenBalanceWithUsdValue,
			AssetID:                  balance.AssetID,
			IsLegacy:                 balance.IsLegacy,
		})
	}
	return stuckBridgeBalances
}

// RecoverStuckBridgeBalance mirrors recoverStuckBridgeBalance (stuck-bridge-balances.utils.ts).
func RecoverStuckBridgeBalance(ctx context.Context, hinkal ihinkal.HinkalInternal, balance types.StuckBridgeBalance, recipientAddress string) error {
	await, err := StartStuckBridgeBalanceRecovery(ctx, hinkal, balance, recipientAddress)
	if err != nil {
		return err
	}
	return await(ctx)
}

func StartStuckBridgeBalanceRecovery(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	balance types.StuckBridgeBalance,
	rawRecipientAddress string,
) (func(context.Context) error, error) {
	recipientAddress := strings.TrimSpace(rawRecipientAddress)
	if recipientAddress != "" && balance.Token.ChainID != 0 {
		var err error
		recipientAddress, err = toBase58IfTron(balance.Token.ChainID, recipientAddress)
		if err != nil {
			return nil, err
		}
	}

	if isNearIntentsRefundBalance(balance) {
		return startNearIntentsRefundBalanceClaim(ctx, hinkal, types.NearIntentsRefundBalance{
			TokenBalanceWithUsdValue: balance.TokenBalanceWithUsdValue,
			AssetID:                  balance.AssetID,
			IsLegacy:                 balance.IsLegacy,
		}, recipientAddress)
	}
	err := recoverTemporaryWalletBalance(ctx, hinkal, types.TemporaryWalletBalance{
		TokenBalanceWithUsdValue: balance.TokenBalanceWithUsdValue,
		Nonce:                    balance.Nonce,
		EthAddress:               balance.EthAddress,
		PrivateKey:               balance.PrivateKey,
		RecoveryDestination:      balance.RecoveryDestination,
	}, recipientAddress)
	if err != nil {
		return nil, err
	}
	return func(context.Context) error { return nil }, nil
}
