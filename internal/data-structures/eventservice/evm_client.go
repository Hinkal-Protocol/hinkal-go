package eventservice

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func NewEVMEmitter(chainID int, rpcURL, contractAddress string, initialBlock uint64, eventsFetchingMutex *sync.RWMutex) (*BlockchainEventEmitter, error) {
	client, err := api.DialEthClientWithFallback(chainID, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}
	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, fmt.Errorf("load abi: %w", err)
	}
	return New(
		chainID,
		client,
		common.HexToAddress(contractAddress),
		parsedABI,
		initialBlock,
		false,
		NewClientBlockchainEventEmitter(types.EventCategoryMain),
		eventsFetchingMutex,
		nil,
	), nil
}
