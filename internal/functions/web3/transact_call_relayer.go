package web3

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/snarkjs"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func extractTxResponse(resp api.RelayerResponse) (string, error) {
	if resp.Status == "success" {
		return resp.Message, nil
	}
	if resp.Message != "" && resp.Error != "" {
		return "", &errorhandling.ErrorWithRelayerTransaction{Message: resp.Error, TxHash: resp.Message}
	}
	return "", errors.New(resp.Error)
}

func extractBatchTxResponse(resp api.RelayerBatchResponse) (string, error) {
	if resp.Status == "success" {
		if resp.Message == nil || resp.Message.ScheduleID == "" {
			return "", errors.New("relayer batch response missing scheduleId")
		}
		return resp.Message.ScheduleID, nil
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	return "", errors.New("batch transaction failed")
}

func SolanaTransactCallRelayer(ctx context.Context, body api.SolanaTransactionBody) (string, error) {
	resp, err := api.CallRelayerSolanaTransactAPI(ctx, body)
	if err != nil {
		return "", err
	}
	return extractTxResponse(resp)
}

type TransactCallRelayerBatchItem struct {
	ZkCallData               types.NewZkCallDataType
	DimData                  types.DimDataType
	CircomData               types.CircomDataType
	CommitmentValidationData *types.CommitmentValidationDataType
	WithUniswapWorkAround    bool
	AuthorizationData        *types.AuthorizationData
	RecipientAddress         string
	AdminData                *types.AdminDataType
	TronProofSignature       *api.TronProofSignature
}

func SolanaTransactCallRelayerBatch(
	ctx context.Context,
	chainID int,
	transactions []api.SolanaTransactionBody,
	hashedEthereumAddress string,
	txCompletionTime *int,
	ref string,
	hashedDashboardAccountID string,
) (string, error) {
	resp, err := api.CallRelayerSolanaTransactBatchAPI(ctx, api.SolanaTransactionBatchRequestBody{
		ChainID:                  chainID,
		Transactions:             transactions,
		HashedEthereumAddress:    hashedEthereumAddress,
		TxCompletionTime:         txCompletionTime,
		Ref:                      ref,
		HashedDashboardAccountID: hashedDashboardAccountID,
	})
	if err != nil {
		return "", err
	}
	return extractBatchTxResponse(resp)
}

func TransactCallRelayer(
	ctx context.Context,
	chainID int,
	zkCallData types.NewZkCallDataType,
	dimData types.DimDataType,
	circomData types.CircomDataType,
	commitmentValidationData *types.CommitmentValidationDataType,
	withUniswapWorkAround bool,
	authorizationData *types.AuthorizationData,
	adminData *types.AdminDataType,
	tronProofSignature *api.TronProofSignature,
) (string, error) {
	resp, err := api.CallRelayerTransactAPI(ctx, api.HinkalTransactionRequestBody{
		ChainID:                  chainID,
		A:                        zkCallData.A,
		B:                        zkCallData.B,
		C:                        zkCallData.C,
		DimData:                  dimData,
		CircomData:               snarkjs.SerializeCircomData(circomData),
		CommitmentValidationData: commitmentValidationData,
		WithUniswapWorkAround:    withUniswapWorkAround,
		AuthorizationData:        authorizationData,
		AdminData:                adminData,
		TronProofSignature:       tronProofSignature,
	})
	if err != nil {
		return "", err
	}
	return extractTxResponse(resp)
}

func TransactCallRelayerBatch(
	ctx context.Context,
	chainID int,
	transactions []TransactCallRelayerBatchItem,
	hashedEthereumAddress string,
	txCompletionTime *int,
	ref string,
	hashedDashboardAccountID string,
) (string, error) {
	if len(transactions) == 0 {
		return "", fmt.Errorf("web3: batch transactions must not be empty")
	}
	bodies := make([]api.HinkalTransactionRequestBody, len(transactions))
	for i, tx := range transactions {
		bodies[i] = api.HinkalTransactionRequestBody{
			ChainID:                  chainID,
			A:                        tx.ZkCallData.A,
			B:                        tx.ZkCallData.B,
			C:                        tx.ZkCallData.C,
			DimData:                  tx.DimData,
			CircomData:               snarkjs.SerializeCircomData(tx.CircomData),
			CommitmentValidationData: tx.CommitmentValidationData,
			WithUniswapWorkAround:    tx.WithUniswapWorkAround,
			AuthorizationData:        tx.AuthorizationData,
			RecipientAddress:         tx.RecipientAddress,
			AdminData:                tx.AdminData,
			TronProofSignature:       tx.TronProofSignature,
		}
	}
	resp, err := api.CallRelayerTransactBatchAPI(ctx, api.HinkalTransactionBatchRequestBody{
		ChainID:                  chainID,
		Transactions:             bodies,
		HashedEthereumAddress:    hashedEthereumAddress,
		TxCompletionTime:         txCompletionTime,
		Ref:                      ref,
		HashedDashboardAccountID: hashedDashboardAccountID,
	})
	if err != nil {
		return "", err
	}
	return extractBatchTxResponse(resp)
}
