package tron

import (
	"context"
	"fmt"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func availableResource(limit, used int64) *big.Int {
	if limit > used {
		return big.NewInt(limit - used)
	}
	return big.NewInt(0)
}

func EstimateTronFeeSunWithPadding(
	ctx context.Context,
	wc *WalletClient,
	ownerBase58, contractBase58 string,
	callData []byte,
	callValueSun *big.Int,
) (*big.Int, error) {
	callValue := int64(0)
	if callValueSun != nil && callValueSun.Sign() > 0 {
		callValue = callValueSun.Int64()
	}

	constantResult, err := wc.TriggerConstantContract(ctx, ownerBase58, contractBase58, callData, callValue)
	if err != nil {
		return nil, fmt.Errorf("tron constant contract call: %w", err)
	}

	usedEnergy := big.NewInt(constantResult.EnergyUsed)
	if required, err := wc.EstimateEnergy(ctx, ownerBase58, contractBase58, callData, callValue); err == nil && required > 0 {
		usedEnergy = big.NewInt(required)
	}

	chainParams, err := wc.GetChainParameters(ctx)
	if err != nil {
		return nil, fmt.Errorf("tron chain parameters: %w", err)
	}
	energyUnitPriceSun, err := getChainParameterValue(chainParams, "getEnergyFee")
	if err != nil {
		return nil, err
	}
	bandwidthUnitPriceSun, err := getChainParameterValue(chainParams, "getTransactionFee")
	if err != nil {
		return nil, err
	}

	ownerHexPrefixed, err := tronHexPrefixed(ownerBase58)
	if err != nil {
		return nil, err
	}
	resources, err := wc.GetAccountResource(ctx, ownerHexPrefixed)
	if err != nil {
		return nil, fmt.Errorf("tron account resources: %w", err)
	}

	availableEnergy := availableResource(resources.EnergyLimit, resources.EnergyUsed)
	payableEnergy := new(big.Int).Sub(usedEnergy, availableEnergy)
	if payableEnergy.Sign() < 0 {
		payableEnergy = big.NewInt(0)
	}
	payableEnergySun := new(big.Int).Mul(payableEnergy, energyUnitPriceSun)

	txSizeBytes := big.NewInt(int64(len(constantResult.Transaction.RawDataHex) / 2))

	freeNetLeft := availableResource(resources.FreeNetLimit, resources.FreeNetUsed)
	stakedNetLeft := availableResource(resources.NetLimit, resources.NetUsed)
	availableBandwidth := new(big.Int).Add(freeNetLeft, stakedNetLeft)
	payableBandwidth := new(big.Int).Sub(txSizeBytes, availableBandwidth)
	if payableBandwidth.Sign() < 0 {
		payableBandwidth = big.NewInt(0)
	}
	payableBandwidthSun := new(big.Int).Mul(payableBandwidth, bandwidthUnitPriceSun)

	estimatedFeeSun := new(big.Int).Add(payableEnergySun, payableBandwidthSun)
	padding := new(big.Int).Mul(estimatedFeeSun, big.NewInt(constants.TronFeePaddingBps))
	padding.Div(padding, big.NewInt(10_000))
	paddedFeeSun := new(big.Int).Add(estimatedFeeSun, padding)

	feeLimit := big.NewInt(constants.TronDefaultFeeLimitSun)
	if paddedFeeSun.Cmp(feeLimit) > 0 {
		return feeLimit, nil
	}
	return paddedFeeSun, nil
}

func getChainParameterValue(params []ChainParameter, key string) (*big.Int, error) {
	for _, p := range params {
		if p.Key == key {
			return big.NewInt(p.Value), nil
		}
	}
	return nil, fmt.Errorf("missing Tron chain parameter: %s", key)
}

// tronHexPrefixed converts a base58/0x address to Tron's 41-prefixed hex form (visible:false), as
// expected by /wallet/getaccountresource. Mirrors the TS fetchTronAccountResources conversion.
func tronHexPrefixed(addr string) (string, error) {
	hexAddr, err := utils.AddressToHexFormat(addr)
	if err != nil {
		return "", err
	}
	return "41" + hexAddr[2:], nil
}
