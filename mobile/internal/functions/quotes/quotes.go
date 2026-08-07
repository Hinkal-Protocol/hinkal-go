package quotes

import (
	"context"
	"encoding/hex"
	"encoding/json"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/integrations"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func GetSwapQuotesJSON(chainID64 int64, inAmount, inTokenAddr, outTokenAddr string) (string, error) {
	if constants.IsSolanaLike(int(chainID64)) {
		return GetSolanaSwapPricesJSON(chainID64, inAmount, inTokenAddr, outTokenAddr)
	}
	return GetEvmSwapPricesJSON(chainID64, inAmount, inTokenAddr, outTokenAddr)
}

func GetEvmSwapPricesJSON(chainID64 int64, inAmount, inTokenAddr, outTokenAddr string) (string, error) {
	prices, err := integrations.GetEVMSwapPrices(context.Background(), int(chainID64), inAmount, inTokenAddr, outTokenAddr)
	if err != nil {
		return "", err
	}
	return codec.EncodeEVMSwapPrices(prices)
}

func GetSolanaSwapPricesJSON(chainID64 int64, inAmount, inTokenAddr, outTokenAddr string) (string, error) {
	prices, err := integrations.GetSolanaSwapPrices(context.Background(), int(chainID64), inAmount, inTokenAddr, outTokenAddr)
	if err != nil {
		return "", err
	}
	return codec.EncodeSolanaSwapPrices(prices)
}

func GetExternalSwapAddress(chainID64 int64, actionID string) (string, error) {
	return pretransaction.GetExternalSwapAddress(int(chainID64), types.ExternalActionID(actionID))
}

func resolveSwapPair(ctx context.Context, chainID int, inTokenAddr, outTokenAddr string) (types.ERC20Token, types.ERC20Token, error) {
	inToken, err := web3.ResolveERC20TokenStrict(ctx, chainID, inTokenAddr)
	if err != nil {
		return types.ERC20Token{}, types.ERC20Token{}, err
	}
	outToken, err := web3.ResolveERC20TokenStrict(ctx, chainID, outTokenAddr)
	if err != nil {
		return types.ERC20Token{}, types.ERC20Token{}, err
	}
	return inToken, outToken, nil
}

func GetOKXQuoteJSON(chainID64 int64, inAmount, inTokenAddr, outTokenAddr string, slippagePercentage float64) (string, error) {
	ctx := context.Background()
	chainID := int(chainID64)
	inToken, outToken, err := resolveSwapPair(ctx, chainID, inTokenAddr, outTokenAddr)
	if err != nil {
		return "", err
	}
	p, err := web3.GetOKXPrice(ctx, chainID, inToken, outToken, inAmount, slippagePercentage)
	if err != nil {
		return "", err
	}
	return codec.JSONString(codec.AppendSwapQuote(make([]mobiletypes.SwapQuoteJSON, 0, 1), types.ExternalActionOkx, p.OutSwapAmount, p.OKXData))
}

func GetLifiBridgeQuoteJSON(
	sourceChainID, destinationChainID int64,
	sourceTokenAddr, destinationTokenAddr string,
	amount string,
	slippage float64,
	fromAddress, toAddress string,
) (string, error) {
	ctx := context.Background()
	sourceToken, err := web3.ResolveERC20TokenStrict(ctx, int(sourceChainID), sourceTokenAddr)
	if err != nil {
		return "", err
	}
	destinationToken, err := web3.ResolveERC20TokenStrict(ctx, int(destinationChainID), destinationTokenAddr)
	if err != nil {
		return "", err
	}
	quote, err := web3.GetLifiPrice(ctx, sourceToken, destinationToken, amount, slippage, fromAddress, toAddress)
	if err != nil {
		return "", err
	}
	return codec.JSONString(codec.EncodeBridgeQuote(quote))
}

func GetNearIntentsQuoteJSON(requestJSON string) (string, error) {
	var req types.NearIntentsQuoteRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", mobileerrors.InvalidJSON("requestJSON", err)
	}
	resp, err := api.GetNearIntentsQuote(context.Background(), req)
	if err != nil {
		return "", err
	}
	return codec.JSONString(resp)
}

func GetNearIntentsTokensJSON() (string, error) {
	list, err := api.GetNearIntentsTokens(context.Background())
	if err != nil {
		return "", err
	}
	return codec.JSONString(list)
}

func NewTemporarySubAccountJSON(index int64) (string, error) {
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		return "", err
	}
	return codec.JSONString(types.TemporarySubAccount{
		Index:      int(index),
		EthAddress: gethcrypto.PubkeyToAddress(key.PublicKey).Hex(),
		PrivateKey: "0x" + hex.EncodeToString(gethcrypto.FromECDSA(key)),
	})
}
