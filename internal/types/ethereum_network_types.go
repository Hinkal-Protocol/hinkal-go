package types

import "math/big"

type ContractType int

const (
	ContractTypeHinkal       ContractType = iota // HinkalContract
	ContractTypeHinkalHelper                     // HinkalHelperContract
	ContractTypeERC20                            // ERC20Contract
	ContractTypeERC721                           // ERC721Contract
	ContractTypeERC1155                          // ERC1155Contract
	ContractTypeWAToken                          // WATokenContract
	ContractTypeMerkleTree                       // MerkleTreeContract
)

type EthereumNetwork struct {
	Name        string
	ChainID     int
	RPCURL      string
	FetchRPCURL string
	WsRPCURL    string
	Supported   bool
	Priority    int
	MaxPageSize int
}

type TransactionRequest struct {
	To       string
	Data     []byte
	Value    *big.Int
	GasLimit uint64
}
