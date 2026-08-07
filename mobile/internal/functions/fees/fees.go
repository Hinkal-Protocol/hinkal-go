package fees

import (
	"context"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	sdkfees "github.com/Hinkal-Protocol/hinkal-go/internal/functions/fees"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func GetFeeStructureJSON(
	chainID64 int64,
	feeTokenAddr, tokenAddrsJSON, actionID string,
	callsJSON string,
	variableRateWei string,
	solanaParamsJSON string,
) (string, error) {
	args, err := codec.DecodeFeeStructureArgs(chainID64, feeTokenAddr, tokenAddrsJSON, callsJSON, variableRateWei, solanaParamsJSON)
	if err != nil {
		return "", err
	}

	fee, err := pretransaction.GetFeeStructure(
		context.Background(),
		args.ChainID,
		feeTokenAddr,
		args.TokenAddrs,
		types.ExternalActionID(actionID),
		args.Calls,
		args.VariableRate,
		args.SolanaParams,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeFeeStructure(fee)
}

func GetGasTokenSymbols(chainID64 int64) (string, error) {
	symbols, err := sdkfees.GetGasTokenSymbols(context.Background(), int(chainID64))
	if err != nil {
		return "", err
	}
	return codec.JSONString(symbols)
}

func CalculateTotalFee(amountWei, feeStructureJSON string) (string, error) {
	amount, err := codec.DecodeBig(amountWei)
	if err != nil {
		return "", err
	}
	feeStructure, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	if feeStructure == nil {
		return "", mobileerrors.ErrFeeStructureRequired
	}
	return codec.EncodeBig(sdkfees.CalculateTotalFee(amount, *feeStructure)), nil
}

func CalculateWithdrawalAmount(amountWithFeeWei, feeStructureJSON string) (string, error) {
	amountWithFee, err := codec.DecodeBig(amountWithFeeWei)
	if err != nil {
		return "", err
	}
	feeStructure, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	if feeStructure == nil {
		return "", mobileerrors.ErrFeeStructureRequired
	}
	return codec.EncodeBig(sdkfees.CalculateWithdrawalAmount(amountWithFee, *feeStructure)), nil
}

func CalculateModifiedFeeStructure(chainID64 int64, tokenAddr, amountWei, feeStructureJSON string) (string, error) {
	chainID := int(chainID64)
	amount, err := codec.DecodeBig(amountWei)
	if err != nil {
		return "", err
	}
	feeStructure, err := codec.DecodeFeeStructure(feeStructureJSON)
	if err != nil {
		return "", err
	}
	if feeStructure == nil {
		return "", mobileerrors.ErrFeeStructureRequired
	}
	amountToken, err := web3.ResolveERC20TokenStrict(context.Background(), chainID, tokenAddr)
	if err != nil {
		return "", err
	}
	modified := sdkfees.CalculateModifiedFeeStructure(context.Background(), chainID, amountToken, amount, *feeStructure)
	return codec.EncodeFeeStructure(modified)
}
