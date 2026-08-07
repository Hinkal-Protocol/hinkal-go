package transactions

import (
	"context"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
)

func submitEvm(ctx context.Context, hinkal ihinkal.HinkalInternal, params HinkalTransactParams, proof TransactProof) (TransactResult, error) {
	switch params.Submit.Mode {
	case SubmitModeProofOnly:
		return TransactResult{Proof: proof}, nil

	case SubmitModeSelf:
		self := params.Submit.Self
		adapter, err := hinkal.GetProviderAdapter(&params.ChainID)
		if err != nil {
			return TransactResult{}, err
		}
		txRequest, txHash, err := web3.TransactCallDirect(ctx, adapter, params.ChainID, web3.TransactCallDirectParams{
			Amounts:         self.ApprovalAmounts,
			TokensToApprove: self.Erc20Tokens,
			ZkCallData:      proof.ZkCallData,
			CircomData:      proof.CircomData,
			DimData:         proof.DimData,
			PreEstimateGas:  self.PreEstimateGas,
			ReturnTxData:    self.ReturnTxData,
		})
		return TransactResult{TxHash: txHash, TxRequest: txRequest, Proof: proof}, err

	case SubmitModeRelayer:
		relayer := params.Submit.Relayer
		txHash, err := web3.TransactCallRelayer(
			ctx, params.ChainID, proof.ZkCallData, proof.DimData, proof.CircomData, proof.CommitmentValidationData,
			relayer.WithUniswapWorkAround, relayer.AuthorizationData, relayer.AdminData, nil,
		)
		return TransactResult{TxHash: txHash, Proof: proof}, err

	// Adding a SubmitMode without handling it here fails at run time.
	default:
		return TransactResult{}, fmt.Errorf("transactions: submitEvm: unhandled submit mode %q", params.Submit.Mode)
	}
}

func submitTron(ctx context.Context, hinkal ihinkal.HinkalInternal, params HinkalTransactParams, proof TransactProof) (TransactResult, error) {
	// TransactCallDirectTron calls ReorderZkCallData itself,
	// so this path must not reorder first or the calldata gets mutated twice.
	if params.Submit.Mode == SubmitModeSelf {
		self := params.Submit.Self
		if self.ReturnTxData {
			return TransactResult{}, errTronReturnTxDataNotImplemented
		}
		client, err := hinkal.GetTronWeb()
		if err != nil {
			return TransactResult{}, err
		}
		txid, err := tron.TransactCallDirectTron(ctx, client, params.ChainID, tron.TransactCallDirectTronParams{
			Amounts:         self.ApprovalAmounts,
			TokensToApprove: self.Erc20Tokens,
			ZkCallData:      proof.ZkCallData,
			CircomData:      proof.CircomData,
			DimData:         proof.DimData,
			PreEstimateGas:  self.PreEstimateGas,
		})
		return TransactResult{TxHash: txid, Proof: proof}, err
	}

	if proof.TronProofSignature != nil {
		tron.SwapTronBCoordinate(&proof.ZkCallData)
	} else {
		signature, err := tron.ReorderZkCallData(ctx, params.ChainID, &proof.ZkCallData, proof.DimData, proof.CircomData, true)
		if err != nil {
			return TransactResult{}, err
		}
		proof.TronProofSignature = &signature
	}

	switch params.Submit.Mode {
	case SubmitModeProofOnly:
		return TransactResult{Proof: proof}, nil

	case SubmitModeRelayer:
		relayer := params.Submit.Relayer
		txHash, err := web3.TransactCallRelayer(
			ctx, params.ChainID, proof.ZkCallData, proof.DimData, proof.CircomData, proof.CommitmentValidationData,
			relayer.WithUniswapWorkAround, relayer.AuthorizationData, relayer.AdminData, proof.TronProofSignature,
		)
		return TransactResult{TxHash: txHash, Proof: proof}, err

	default:
		return TransactResult{}, fmt.Errorf("transactions: submitTron: unhandled submit mode %q", params.Submit.Mode)
	}
}

func SubmitProof(ctx context.Context, hinkal ihinkal.HinkalInternal, params HinkalTransactParams, proof TransactProof) (TransactResult, error) {
	if constants.IsTronLike(params.ChainID) {
		return submitTron(ctx, hinkal, params, proof)
	}
	return submitEvm(ctx, hinkal, params, proof)
}
