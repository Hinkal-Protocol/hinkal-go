package web3

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

var (
	fetchClientsMu sync.Mutex
	fetchClients   = map[int]*ethclient.Client{}
)

func fetchClient(chainID int) (*ethclient.Client, error) {
	fetchClientsMu.Lock()
	defer fetchClientsMu.Unlock()

	if client, ok := fetchClients[chainID]; ok {
		return client, nil
	}
	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return nil, err
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	fetchClients[chainID] = client
	return client, nil
}
