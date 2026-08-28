package tron

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errTokenAmountMismatch     = errors.New("tron: token and amount length mismatch")
	ErrInsufficientTronBalance = errors.New("INSUFFICIENT_TRON_BALANCE_FOR_FEE")
)

type TronClient interface {
	GetAddress() string
	WalletClient() *WalletClient
	SignAndBroadcast(ctx context.Context, tx *Transaction) (string, error)
}

type TransactCallDirectTronParams struct {
	Amounts                   []*big.Int
	TokensToApprove           []types.ERC20Token // erc20TokenAddress in hex format
	ZkCallData                types.NewZkCallDataType
	CircomData                types.CircomDataType
	DimData                   types.DimDataType
	ContractApproval          string // base58 or hex; defaults to the Hinkal contract
	ContractTransact          string // base58 or hex; defaults to the Hinkal contract
	PreEstimateGas            bool
	PrecomputedProofSignature *api.TronProofSignature
}

func hexTo32(s string) ([32]byte, error) {
	b := common.FromHex(s)
	var out [32]byte
	if len(b) != 32 {
		return out, fmt.Errorf("tron: expected 32-byte value, got %d", len(b))
	}
	copy(out[:], b)
	return out, nil
}

func TransactCallDirectTron(ctx context.Context, client TronClient, chainID int, params TransactCallDirectTronParams) (string, error) {
	if len(params.Amounts) != len(params.TokensToApprove) {
		return "", errTokenAmountMismatch
	}

	wc := client.WalletClient()
	ownerBase58 := client.GetAddress()
	ownerHex, err := utils.AddressToHexFormat(ownerBase58)
	if err != nil {
		return "", err
	}

	hinkalHex, err := constants.HinkalAddress(chainID)
	if err != nil {
		return "", err
	}
	hinkalBase58, err := utils.EVMHexToTronBase58Address(hinkalHex)
	if err != nil {
		return "", err
	}

	contractForApproval := hinkalBase58
	if params.ContractApproval != "" {
		if contractForApproval, err = utils.ToTronBase58IfHex(params.ContractApproval); err != nil {
			return "", err
		}
	}
	contractForTransaction := hinkalBase58
	if params.ContractTransact != "" {
		if contractForTransaction, err = utils.ToTronBase58IfHex(params.ContractTransact); err != nil {
			return "", err
		}
	}

	callValueSun, approvals, err := SplitTokensByNative(params.TokensToApprove, params.Amounts)
	if err != nil {
		return "", err
	}

	if err := ApproveTokens(ctx, client, chainID, ownerBase58, ownerHex, contractForApproval, approvals); err != nil {
		return "", err
	}

	proofSig := api.TronProofSignature{}
	if params.PrecomputedProofSignature != nil {
		proofSig = *params.PrecomputedProofSignature
	} else {
		// modify=false: the contract takes the b coordinates in the order the proof was built with.
		proofSig, err = ReorderZkCallData(ctx, chainID, &params.ZkCallData, params.DimData, params.CircomData, false, false)
		if err != nil {
			return "", err
		}
	}
	proofR, err := hexTo32(proofSig.R)
	if err != nil {
		return "", err
	}
	proofS, err := hexTo32(proofSig.S)
	if err != nil {
		return "", err
	}

	callData, err := contractabi.PackTronTransact(chainID, uint8(proofSig.V), proofR, proofS, params.ZkCallData, params.DimData, params.CircomData)
	if err != nil {
		return "", err
	}

	if params.PreEstimateGas {
		if err := SimulateTronTransaction(ctx, chainID, contractForTransaction, ownerBase58, callData, callValueSun); err != nil {
			return "", err
		}

		senderBalanceSun, err := wc.GetAccountBalanceSun(ctx, ownerBase58)
		if err != nil {
			return "", fmt.Errorf("tron get account: %w", err)
		}

		estimatedFeeSun := big.NewInt(constants.TronDefaultFeeLimitSun)
		if fee, err := EstimateTronFeeSunWithPadding(ctx, wc, ownerBase58, contractForTransaction, callData, callValueSun); err == nil {
			estimatedFeeSun = fee
		}
		totalRequiredSun := new(big.Int).Add(callValueSun, estimatedFeeSun)
		if senderBalanceSun.Cmp(totalRequiredSun) < 0 {
			return "", ErrInsufficientTronBalance
		}
	}

	sendTx, err := wc.TriggerSmartContract(ctx, ownerBase58, contractForTransaction, callData, callValueSun.Int64(), constants.TronDefaultFeeLimitSun)
	if err != nil {
		return "", fmt.Errorf("tron build transact: %w", err)
	}
	txid, err := client.SignAndBroadcast(ctx, sendTx)
	if err != nil {
		return "", err
	}
	return txid, nil
}
