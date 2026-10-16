package pretransaction

import (
	"runtime"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SdkFunction string

const (
	SdkDeposit                   SdkFunction = "deposit"
	SdkDepositForOther           SdkFunction = "deposit_for_other"
	SdkDepositSolana             SdkFunction = "deposit_solana"
	SdkDepositSolanaForOther     SdkFunction = "deposit_solana_for_other"
	SdkProoflessDeposit          SdkFunction = "proofless_deposit"
	SdkWithdraw                  SdkFunction = "withdraw"
	SdkTransfer                  SdkFunction = "transfer"
	SdkDepositAndWithdraw        SdkFunction = "deposit_and_withdraw"
	SdkDepositAndBridge          SdkFunction = "deposit_and_bridge"
	SdkDepositAndBridgeWithQuote SdkFunction = "deposit_and_bridge_with_quote"
	SdkNearDepositAndBridge      SdkFunction = "near_deposit_and_bridge"
	SdkBridgePrivateToPrivate    SdkFunction = "bridge_private_to_private"
	SdkSwap                      SdkFunction = "swap"
	SdkProxyToPrivate            SdkFunction = "proxy_to_private"
	SdkSolanaSponsoredShield     SdkFunction = "solana_sponsored_shield"
	SdkSolanaSponsoredProxySend  SdkFunction = "solana_sponsored_proxy_send"
)

var sdkFunctionsWithoutOwner = map[SdkFunction]bool{
	SdkWithdraw:               true,
	SdkTransfer:               true,
	SdkBridgePrivateToPrivate: true,
}

func sdkProductPrefix() string {
	return sdkProductPrefixForOS(runtime.GOOS)
}

func sdkProductPrefixForOS(goos string) string {
	switch goos {
	case "android":
		return "go_android_sdk"
	case "ios":
		return "go_ios_sdk"
	default:
		return "go_sdk"
	}
}

func SdkAction(fn SdkFunction) types.AdminTransactionType {
	return types.AdminTransactionType(sdkProductPrefix() + "_" + string(fn))
}

func isPrivateTransferOrWithdraw(action types.AdminTransactionType) bool {
	return sdkFunctionsWithoutOwner[SdkFunction(strings.TrimPrefix(string(action), sdkProductPrefix()+"_"))]
}
