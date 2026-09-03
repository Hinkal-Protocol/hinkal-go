package fees

import (
	"context"
	"errors"
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errFailedTokenPrices = errors.New("failed to get token prices")

func GetGasTokenSymbols(ctx context.Context, chainID int) ([]string, error) {
	if chainID == 0 {
		return constants.ExtendedNonNativeGasCostTokenSymbolOptions, nil
	}

	nativeTokenAddress := constants.ZeroAddress
	if constants.IsSolanaLike(chainID) {
		nativeTokenAddress = constants.SolanaNativeAddress
	}
	var nativeTokenSymbol string
	nativeToken, err := web3.GetErc20TokenFromAPI(ctx, chainID, nativeTokenAddress)
	if err != nil {
		return nil, err
	}
	if nativeToken != nil {
		nativeTokenSymbol = nativeToken.Symbol
	}

	if constants.IsSolanaLike(chainID) && nativeTokenSymbol != "" {
		return []string{nativeTokenSymbol}, nil
	}
	options := constants.NonNativeGasCostTokenSymbolOptions(chainID)
	if nativeTokenSymbol != "" && !containsString(options, nativeTokenSymbol) {
		return append([]string{nativeTokenSymbol}, options...), nil
	}
	return options, nil
}

func CalculateTotalFee(amount *big.Int, feeStructure types.FeeStructure) *big.Int {
	variableFee := new(big.Int).Quo(new(big.Int).Mul(amount, feeStructure.VariableRate), constants.BPSDenominator())
	return new(big.Int).Add(feeStructure.FlatFee, variableFee)
}

func CalculateModifiedFeeStructure(
	ctx context.Context,
	chainID int,
	amountToken types.ERC20Token,
	amount *big.Int,
	feeStructure types.FeeStructure,
) types.FeeStructure {
	feeToken, err := web3.GetErc20TokenFromAPI(ctx, chainID, feeStructure.FeeToken)
	if err != nil {
		log.Printf("calculateModifiedFeeStructure: failed to resolve fee token, charging gas fee only: %v", err)
		feeToken = nil
	}

	var totalFee *big.Int
	if feeToken == nil ||
		strings.EqualFold(feeStructure.FeeToken, amountToken.Erc20TokenAddress) ||
		feeStructure.VariableRate.Sign() == 0 {
		totalFee = CalculateTotalFee(amount, feeStructure)
	} else {
		variableFee := big.NewInt(0)
		if vf, err := crossTokenVariableFee(ctx, chainID, amountToken, amount, feeStructure, *feeToken); err != nil {
			log.Printf("calculateModifiedFeeStructure: failed to price cross-token variable fee, charging gas fee only: %v", err)
		} else {
			variableFee = vf
		}
		totalFee = new(big.Int).Add(feeStructure.FlatFee, variableFee)
	}

	return types.FeeStructure{
		FeeToken:     feeStructure.FeeToken,
		FlatFee:      totalFee,
		VariableRate: big.NewInt(0),
	}
}

func crossTokenVariableFee(
	ctx context.Context,
	chainID int,
	amountToken types.ERC20Token,
	amount *big.Int,
	feeStructure types.FeeStructure,
	feeToken types.ERC20Token,
) (*big.Int, error) {
	amountTokenPrice, feeTokenPrice, err := tokenPricePair(ctx, chainID, amountToken, feeToken)
	if err != nil {
		return nil, err
	}

	amountInToken, err := strconv.ParseFloat(web3.GetAmountInToken(amountToken, amount), 64)
	if err != nil {
		return nil, err
	}
	amountValueUsd := amountInToken * amountTokenPrice
	variableFeeUsd := amountValueUsd * float64(feeStructure.VariableRate.Int64()) / float64(constants.BPSDenominator().Int64())
	weiInput := strconv.FormatFloat(variableFeeUsd/feeTokenPrice, 'f', feeToken.Decimals, 64)
	return web3.GetAmountInWei(feeToken, weiInput)
}

func tokenPricePair(
	ctx context.Context,
	chainID int,
	amountToken types.ERC20Token,
	feeToken types.ERC20Token,
) (float64, float64, error) {
	if amountToken.ChainID == chainID {
		resp, err := api.GetTokenPrices(ctx, chainID, []string{amountToken.Erc20TokenAddress, feeToken.Erc20TokenAddress})
		if err != nil {
			return 0, 0, err
		}
		if len(resp.Prices) < 2 || resp.Prices[0] == 0 || resp.Prices[1] == 0 {
			return 0, 0, errFailedTokenPrices
		}
		return resp.Prices[0], resp.Prices[1], nil
	}

	amountResp, err := api.GetTokenPrices(ctx, amountToken.ChainID, []string{amountToken.Erc20TokenAddress})
	if err != nil {
		return 0, 0, err
	}
	feeResp, err := api.GetTokenPrices(ctx, chainID, []string{feeToken.Erc20TokenAddress})
	if err != nil {
		return 0, 0, err
	}
	if len(amountResp.Prices) == 0 || len(feeResp.Prices) == 0 || amountResp.Prices[0] == 0 || feeResp.Prices[0] == 0 {
		return 0, 0, errFailedTokenPrices
	}
	return amountResp.Prices[0], feeResp.Prices[0], nil
}

func CalculateWithdrawalAmount(amountWithFee *big.Int, feeStructure types.FeeStructure) *big.Int {
	positiveAmountWithFee := new(big.Int).Neg(amountWithFee)

	if feeStructure.VariableRate.Sign() == 0 {
		return new(big.Int).Sub(positiveAmountWithFee, feeStructure.FlatFee)
	}

	numerator := new(big.Int).Mul(new(big.Int).Sub(positiveAmountWithFee, feeStructure.FlatFee), constants.BPSDenominator())
	denominator := new(big.Int).Add(constants.BPSDenominator(), feeStructure.VariableRate)
	return new(big.Int).Quo(numerator, denominator)
}

func CalculateGrossAmountWithFee(netAmount *big.Int, feeStructure types.FeeStructure) *big.Int {
	if feeStructure.VariableRate.Sign() == 0 {
		return new(big.Int).Add(netAmount, feeStructure.FlatFee)
	}
	numerator := new(big.Int).Mul(new(big.Int).Add(netAmount, feeStructure.FlatFee), constants.BPSDenominator())
	denominator := new(big.Int).Sub(constants.BPSDenominator(), feeStructure.VariableRate)
	numerator.Add(numerator, new(big.Int).Sub(denominator, big.NewInt(1)))
	return numerator.Quo(numerator, denominator)
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
