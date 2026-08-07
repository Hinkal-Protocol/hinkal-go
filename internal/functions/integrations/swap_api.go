package integrations

import (
	"context"
	"math/big"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
)

const defaultSwapQuoteTimeout = 30 * time.Second

const defaultEVMSwapSlippage = 0.007

type EVMSwapPrice struct {
	OutSwapAmountValue *big.Int `json:"outSwapAmountValue"`
	LifiDataValue      string   `json:"lifiDataValue"`
}

type SolanaSwapPrice struct {
	OutSwapAmountValue *big.Int `json:"outSwapAmountValue"`
	OKXDataValue       string   `json:"okxDataValue"`
}

func GetEVMSwapPrices(
	ctx context.Context,
	chainID int,
	inSwapAmount string,
	inSwapTokenAddress string,
	outSwapTokenAddress string,
) (*EVMSwapPrice, error) {
	tokens, err := web3.ResolveERC20TokensStrict(ctx, chainID, []string{inSwapTokenAddress, outSwapTokenAddress})
	if err != nil {
		return nil, err
	}
	inSwapToken, outSwapToken := tokens[0], tokens[1]

	quoteCtx, cancel := context.WithTimeout(ctx, defaultSwapQuoteTimeout)
	defer cancel()

	contractData, err := constants.GetContractData(chainID)
	if err != nil || contractData.LifiExternalActionInstanceAddress == "" {
		return nil, nil
	}
	fromAddress := contractData.LifiExternalActionInstanceAddress

	quote, err := web3.GetLifiPrice(quoteCtx, inSwapToken, outSwapToken, inSwapAmount, defaultEVMSwapSlippage, fromAddress, fromAddress)
	if err != nil {
		return nil, nil
	}
	return &EVMSwapPrice{OutSwapAmountValue: quote.ExpectedAmount, LifiDataValue: quote.Calldata}, nil
}

func GetSolanaSwapPrices(
	ctx context.Context,
	chainID int,
	inSwapAmount string,
	inSwapTokenAddress string,
	outSwapTokenAddress string,
) (*SolanaSwapPrice, error) {
	tokens, err := web3.ResolveERC20TokensStrict(ctx, chainID, []string{inSwapTokenAddress, outSwapTokenAddress})
	if err != nil {
		return nil, err
	}

	quoteCtx, cancel := context.WithTimeout(ctx, defaultSwapQuoteTimeout)
	defer cancel()

	price, err := web3.GetOKXPrice(quoteCtx, chainID, tokens[0], tokens[1], inSwapAmount, 0.5)
	if err != nil {
		return nil, nil
	}
	return &SolanaSwapPrice{OutSwapAmountValue: price.OutSwapAmount, OKXDataValue: price.OKXData}, nil
}
