package snarkjs

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
)

func FetchOnChainRootHashes(ctx context.Context, chainID int) (*big.Int, error) {
	rootHash, _, err := FetchOnChainRootHashAndIndex(ctx, chainID)
	return rootHash, err
}

func FetchOnChainRootHashAndIndex(ctx context.Context, chainID int) (*big.Int, *big.Int, error) {
	hinkalAddress, err := constants.HinkalAddress(chainID)
	if err != nil {
		return nil, nil, err
	}
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, nil, err
	}

	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := constants.OriginalDeployer(chainID)
		if err != nil {
			return nil, nil, err
		}
		client := api.NewSolanaClientWithFallback(rpcURL)
		rootHash, err := solana.FetchMerkleTreeRootHash(ctx, client, hinkalAddress, originalDeployer)
		if err != nil {
			return nil, nil, err
		}
		return rootHash, big.NewInt(0), nil
	}

	if constants.IsTronLike(chainID) {
		rootHash, err := tron.FetchRootHash(ctx, chainID)
		if err != nil {
			return nil, nil, err
		}
		return rootHash, big.NewInt(0), nil
	}

	client, err := api.DialEthClientWithFallback(chainID, rpcURL)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, nil, err
	}
	address := common.HexToAddress(hinkalAddress)

	rootHash, err := callHinkalBigIntMethod(ctx, client, parsedABI, address, "getRootHash")
	if err != nil {
		return nil, nil, err
	}
	onChainIndex, err := callHinkalBigIntMethod(ctx, client, parsedABI, address, "m_index")
	if err != nil {
		return nil, nil, err
	}
	rootHashIndex := new(big.Int).Sub(onChainIndex, big.NewInt(1))
	return rootHash, rootHashIndex, nil
}

func callHinkalBigIntMethod(ctx context.Context, client *ethclient.Client, parsedABI abi.ABI, address common.Address, method string) (*big.Int, error) {
	data, err := parsedABI.Pack(method)
	if err != nil {
		return nil, err
	}
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	results, err := parsedABI.Unpack(method, out)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("fetchOnChainRootHashes: empty result for %s", method)
	}
	value, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("fetchOnChainRootHashes: unexpected type %T for %s", results[0], method)
	}
	return value, nil
}
