package receivevault

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errTronReturnTxDataNotImplemented = errors.New("receivevault: Tron returnTxData is not implemented")

func packRecover(
	chainID int,
	params types.ReceiveVaultParams,
	token, to string,
	deadline *big.Int,
	signature string,
) ([]byte, error) {
	factoryABI, err := contractabi.ReceiveVaultFactory(chainID)
	if err != nil {
		return nil, err
	}
	tuple, err := ReceiveVaultParamsToTuple(params)
	if err != nil {
		return nil, err
	}
	tokenHex, err := utils.AddressToHexFormat(token)
	if err != nil {
		return nil, err
	}
	toHex, err := utils.AddressToHexFormat(to)
	if err != nil {
		return nil, err
	}
	return factoryABI.Pack(
		"recover",
		tuple,
		common.HexToAddress(tokenHex),
		common.HexToAddress(toHex),
		deadline,
		common.FromHex(signature),
	)
}

// HinkalReceiveVaultRecover sweeps a receive address the relayer would not settle. The owner's own
// wallet pays for it: the relayer declined these funds, so it must not move them. With returnTxData
// the call is only built, for a caller that broadcasts it itself.
// chainID is where the funds sit; senders miss the chain, and only its own factory can sweep it.
// Zero falls back to a canonical chain in the record's network.
func HinkalReceiveVaultRecover(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	record types.ReceiveVaultRecord,
	token, to string,
	returnTxData bool,
	chainID int,
) (types.TransactionRequest, string, error) {
	if chainID == 0 {
		canonicalChainID, err := getReceiveVaultCanonicalChainID(record.Network)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		chainID = canonicalChainID
	}
	factoryAddress := GetReceiveVaultFactoryAddress(chainID)
	if factoryAddress == "" {
		return types.TransactionRequest{}, "", fmt.Errorf("receive vault: no factory on chain %d", chainID)
	}
	if record.Network != ReceiveVaultNetworkOf(chainID) {
		return types.TransactionRequest{}, "", errors.New("receive vault: network mismatch")
	}

	userKeys := hinkal.GetUserKeys()
	params, _, err := ResolveReceiveVaultParams(ctx, userKeys, record, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	deadline := big.NewInt(utils.GetCurrentTimeInSeconds() + constants.ReceiveVaultRecoveryTTLSeconds)
	signature, err := SignReceiveVaultRecovery(ctx, userKeys, record, token, to, deadline, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := solanaOriginalDeployer(chainID)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		result, err := recoverSolanaReceiveVault(
			ctx,
			hinkal,
			params,
			token,
			to,
			deadline,
			signature,
			factoryAddress,
			originalDeployer,
			returnTxData,
		)
		return types.TransactionRequest{}, result, err
	}
	callData, err := packRecover(chainID, params, token, to, deadline, signature)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	if constants.IsTronLike(chainID) {
		if returnTxData {
			return types.TransactionRequest{}, "", errTronReturnTxDataNotImplemented
		}
		txHash, err := sendTronRecover(ctx, hinkal, factoryAddress, callData)
		return types.TransactionRequest{}, txHash, err
	}

	txReq := types.TransactionRequest{To: factoryAddress, Data: callData}
	if returnTxData {
		return txReq, "", nil
	}

	adapter, err := hinkal.GetProviderAdapter(&chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	txResp, err := adapter.SendTransaction(ctx, txReq)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return txReq, txResp.Hash, nil
}

func sendTronRecover(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	factoryAddress string,
	callData []byte,
) (string, error) {
	client, err := hinkal.GetTronWeb()
	if err != nil {
		return "", err
	}
	factoryBase58, err := utils.ToTronBase58IfHex(factoryAddress)
	if err != nil {
		return "", err
	}

	tx, err := client.WalletClient().TriggerSmartContract(
		ctx,
		client.GetAddress(),
		factoryBase58,
		callData,
		0,
		constants.TronDefaultFeeLimitSun,
	)
	if err != nil {
		return "", fmt.Errorf("tron build receive vault recover: %w", err)
	}
	return client.SignAndBroadcast(ctx, tx)
}
