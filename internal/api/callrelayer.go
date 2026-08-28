package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type TronProofSignature struct {
	V int    `json:"v"`
	R string `json:"r"`
	S string `json:"s"`
}

type HinkalTransactionRequestBody struct {
	ChainID                  int                                 `json:"chainId"`
	A                        [2]string                           `json:"a"`
	B                        [2][2]string                        `json:"b"`
	C                        [2]string                           `json:"c"`
	DimData                  types.DimDataType                   `json:"dimData"`
	CircomData               types.CircomDataJSONType            `json:"circomData"`
	CommitmentValidationData *types.CommitmentValidationDataType `json:"commitmentValidationData,omitempty"`
	WithUniswapWorkAround    bool                                `json:"withUniswapWorkAround,omitempty"`
	AuthorizationData        *types.AuthorizationData            `json:"authorizationData,omitempty"`
	RecipientAddress         string                              `json:"recipientAddress,omitempty"`
	AdminData                *types.AdminDataType                `json:"adminData,omitempty"`
	TronProofSignature       *TronProofSignature                 `json:"tronProofSignature,omitempty"`
}

type RelayerResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type HinkalTransactionBatchRequestBody struct {
	ChainID                  int                            `json:"chainId"`
	Transactions             []HinkalTransactionRequestBody `json:"transactions"`
	HashedEthereumAddress    string                         `json:"hashedEthereumAddress"`
	TxCompletionTime         *int                           `json:"txCompletionTime,omitempty"`
	Ref                      string                         `json:"ref,omitempty"`
	HashedDashboardAccountID string                         `json:"hashedDashboardAccountId,omitempty"`
	DependsOnTxHash          string                         `json:"dependsOnTxHash,omitempty"`
}

type RelayerBatchMessage struct {
	ScheduleID string `json:"scheduleId"`
}

type RelayerBatchResponse struct {
	Status  string               `json:"status"`
	Message *RelayerBatchMessage `json:"message,omitempty"`
	Error   string               `json:"error,omitempty"`
}

type emitTxPublicDataBody struct {
	AdminData *types.AdminDataType `json:"adminData"`
}

func EmitTxPublicData(ctx context.Context, adminData *types.AdminDataType) error {
	if adminData == nil {
		return nil
	}
	return Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.EmitTxPublicData, emitTxPublicDataBody{AdminData: adminData}, nil)
}

func CallRelayerTransactAPI(ctx context.Context, body HinkalTransactionRequestBody) (RelayerResponse, error) {
	var resp RelayerResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.Transact, body, &resp); err != nil {
		return RelayerResponse{}, err
	}
	return resp, nil
}

func CallRelayerTransactBatchAPI(ctx context.Context, body HinkalTransactionBatchRequestBody) (RelayerBatchResponse, error) {
	var resp RelayerBatchResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.TransactBatch, body, &resp); err != nil {
		return RelayerBatchResponse{}, err
	}
	return resp, nil
}
