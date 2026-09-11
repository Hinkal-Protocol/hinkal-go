package web3

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
)

func callReceiveVaultFactory(ctx context.Context, chainID int, data []byte) ([]byte, error) {
	client, err := fetchClient(chainID)
	if err != nil {
		return nil, err
	}
	address, err := constants.ReceiveVaultFactoryAddress(chainID)
	if err != nil {
		return nil, err
	}
	if address == "" {
		return nil, fmt.Errorf("web3: no receive vault factory on chain %d", chainID)
	}
	factory := common.HexToAddress(address)
	return client.CallContract(ctx, ethereum.CallMsg{To: &factory, Data: data}, nil)
}

// ReceiveVaultFactoryVaultVersionCount reads factory.vaultVersionCount() via eth_call.
func ReceiveVaultFactoryVaultVersionCount(ctx context.Context, chainID int) (*big.Int, error) {
	factoryABI, err := contractabi.ReceiveVaultFactory(chainID)
	if err != nil {
		return nil, err
	}
	data, err := factoryABI.Pack("vaultVersionCount")
	if err != nil {
		return nil, err
	}
	out, err := callReceiveVaultFactory(ctx, chainID, data)
	if err != nil {
		return nil, err
	}
	values, err := factoryABI.Unpack("vaultVersionCount", out)
	if err != nil {
		return nil, err
	}
	count, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("web3: unexpected vaultVersionCount type %T", values[0])
	}
	return count, nil
}

// ReceiveVaultFactoryVaultInitCodeHash reads factory.vaultInitCodeHash(version) via eth_call.
func ReceiveVaultFactoryVaultInitCodeHash(ctx context.Context, chainID int, version *big.Int) (string, error) {
	factoryABI, err := contractabi.ReceiveVaultFactory(chainID)
	if err != nil {
		return "", err
	}
	data, err := factoryABI.Pack("vaultInitCodeHash", version)
	if err != nil {
		return "", err
	}
	out, err := callReceiveVaultFactory(ctx, chainID, data)
	if err != nil {
		return "", err
	}
	values, err := factoryABI.Unpack("vaultInitCodeHash", out)
	if err != nil {
		return "", err
	}
	hash, ok := values[0].([32]byte)
	if !ok {
		return "", fmt.Errorf("web3: unexpected vaultInitCodeHash type %T", values[0])
	}
	return "0x" + common.Bytes2Hex(hash[:]), nil
}
