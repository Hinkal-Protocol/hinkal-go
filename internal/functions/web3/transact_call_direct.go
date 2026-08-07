package web3

import (
	"context"
	"errors"
	"math/big"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errTokenAmountMismatch = errors.New("web3: token and amount length mismatch")

type TransactCallDirectParams struct {
	Amounts          []*big.Int
	TokensToApprove  []types.ERC20Token
	ZkCallData       types.NewZkCallDataType
	CircomData       types.CircomDataType
	DimData          types.DimDataType
	ContractApproval string // defaults to the Hinkal contract
	ContractTransact string // defaults to the Hinkal contract
	PreEstimateGas   bool
	ReturnTxData     bool
}

// TransactCallDirect approves the required tokens, then calls (or builds) the Hinkal transact tx.
// When ReturnTxData is set the unsent TransactionRequest is returned and txHash is empty; otherwise
// the tx is sent and its hash returned. The EIP-5792 batch-call branch is intentionally not ported.
func TransactCallDirect(ctx context.Context, adapter types.IProviderAdapter, chainID int, params TransactCallDirectParams) (types.TransactionRequest, string, error) {
	if len(params.Amounts) != len(params.TokensToApprove) {
		return types.TransactionRequest{}, "", errTokenAmountMismatch
	}

	hinkalAddr, err := constants.HinkalAddress(chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	contractForApproval := params.ContractApproval
	if contractForApproval == "" {
		contractForApproval = hinkalAddr
	}
	contractForTransaction := params.ContractTransact
	if contractForTransaction == "" {
		contractForTransaction = hinkalAddr
	}

	ethAddr, err := adapter.GetAddress(ctx)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	ethAmount := big.NewInt(0)
	needsApproval := false
	for i, token := range params.TokensToApprove {
		if token.Erc20TokenAddress == constants.ZeroAddress {
			ethAmount = params.Amounts[i]
		} else {
			needsApproval = true
		}
	}

	if !params.ReturnTxData {
		if err := ApproveTokensToHinkal(ctx, adapter, chainID, common.HexToAddress(contractForApproval), params.TokensToApprove, params.Amounts); err != nil {
			return types.TransactionRequest{}, "", err
		}
		if needsApproval {
			requirements := make([]ApprovalRequirement, len(params.TokensToApprove))
			for i, token := range params.TokensToApprove {
				requirements[i] = ApprovalRequirement{TokenAddress: token.Erc20TokenAddress, RequiredAmount: params.Amounts[i]}
			}
			if err := WaitForErc20Approvals(ctx, adapter, chainID, common.HexToAddress(ethAddr), common.HexToAddress(contractForApproval), requirements, 30, time.Second); err != nil {
				return types.TransactionRequest{}, "", err
			}
		}
	}

	data, err := contractabi.PackTransact(chainID, params.ZkCallData, params.DimData, params.CircomData)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	var value *big.Int
	if ethAmount.Sign() > 0 {
		value = ethAmount
	}

	txReq := types.TransactionRequest{To: contractForTransaction, Data: data, Value: value}

	if params.ReturnTxData {
		return txReq, "", nil
	}

	if params.PreEstimateGas {
		if client, err := adapter.GetFetchClient(chainID); err == nil {
			to := common.HexToAddress(contractForTransaction)
			gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
				From:  common.HexToAddress(ethAddr),
				To:    &to,
				Value: value,
				Data:  data,
			})
			if err == nil {
				txReq.GasLimit = (gas*12 + 9) / 10
			}
		}
	}

	time.Sleep(time.Second) // because public rpc link gives 429

	txResp, err := adapter.SendTransaction(ctx, txReq)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return txReq, txResp.Hash, nil
}
