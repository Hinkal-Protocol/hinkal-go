package api

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type OKXAccount struct {
	IsSigner   bool   `json:"isSigner"`
	IsWritable bool   `json:"isWritable"`
	Pubkey     string `json:"pubkey"`
}

type SolanaArgs struct {
	ProofAArr        []int             `json:"proofAArr"`
	ProofBArr        []int             `json:"proofBArr"`
	ProofCArr        []int             `json:"proofCArr"`
	PublicInputsArr  [][]int           `json:"publicInputsArr"`
	EncryptedOutputs [][]int           `json:"encryptedOutputs"`
	RelayerFee       string            `json:"relayerFee"`
	VariableRate     *string           `json:"variableRate,omitempty"`
	Dimensions       types.DimDataType `json:"dimensions"`
	RootHashIndex    string            `json:"rootHashIndex"`
}

type SolanaHinkalInstruction struct {
	AccountIndexes []int `json:"accountIndexes"`
	Data           []int `json:"data"`
	ProgramIndex   int   `json:"programIndex"`
}

type SolanaSwapArgs struct {
	SolanaArgs
	OnChainEncryptedOutput string                    `json:"onChainEncryptedOutput"`
	OnChainCreation        []bool                    `json:"onChainCreation"`
	HinkalInstructions     []SolanaHinkalInstruction `json:"hinkalInstructions"`
}

type SolanaTransactAccounts struct {
	Recipient string `json:"recipient"`
	Mint      string `json:"mint,omitempty"`
}

type SolanaGasEstimateParams struct {
	MintTo         string `json:"mintTo"`
	MintFrom       string `json:"mintFrom,omitempty"`
	Recipient      string `json:"recipient,omitempty"`
	NullifierCount int    `json:"nullifierCount"`
}

type SolanaSwapAccounts struct {
	Recipient                 string       `json:"recipient"`
	StorageAccount            string       `json:"storageAccount"`
	StorageVault              string       `json:"storageVault"`
	SwapperAccount            string       `json:"swapperAccount"`
	MintFrom                  *string      `json:"mintFrom"`
	MintTo                    *string      `json:"mintTo"`
	RemainingAccounts         []OKXAccount `json:"remainingAccounts"`
	AddressLookupTableAccount []string     `json:"addressLookupTableAccount"`
}

type SolanaTransactionBody struct {
	ChainID                  int                                 `json:"chainId"`
	RelayAddress             string                              `json:"relayAddress"`
	FunctionName             string                              `json:"functionName"` // "transact" | "transfer" | "swap"
	Args                     any                                 `json:"args"`
	Accounts                 any                                 `json:"accounts"`
	CommitmentValidationData *types.CommitmentValidationDataType `json:"commitmentValidationData,omitempty"`
	RecipientAmount          string                              `json:"recipientAmount,omitempty"`
	DisplayRecipient         string                              `json:"displayRecipient,omitempty"`
	AdminData                *types.AdminDataType                `json:"adminData,omitempty"`
}

type SolanaTransactionBatchRequestBody struct {
	ChainID                  int                     `json:"chainId"`
	Transactions             []SolanaTransactionBody `json:"transactions"`
	HashedEthereumAddress    string                  `json:"hashedEthereumAddress"`
	TxCompletionTime         *int                    `json:"txCompletionTime,omitempty"`
	Ref                      string                  `json:"ref,omitempty"`
	HashedDashboardAccountID string                  `json:"hashedDashboardAccountId,omitempty"`
	DependsOnTxHash          string                  `json:"dependsOnTxHash,omitempty"`
}

func CallRelayerSolanaTransactAPI(ctx context.Context, body SolanaTransactionBody) (RelayerResponse, error) {
	var resp RelayerResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.SolanaTransact, body, &resp); err != nil {
		return RelayerResponse{}, err
	}
	return resp, nil
}

func CallRelayerSolanaTransactBatchAPI(ctx context.Context, body SolanaTransactionBatchRequestBody) (RelayerBatchResponse, error) {
	var resp RelayerBatchResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.SolanaTransactBatch, body, &resp); err != nil {
		return RelayerBatchResponse{}, err
	}
	return resp, nil
}
