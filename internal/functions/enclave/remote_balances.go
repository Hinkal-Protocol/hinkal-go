package enclave

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func GetRemoteManagedTokenBalances(
	ctx context.Context,
	chainID int,
	uk *cryptokeys.UserKeys,
	useBlockedUtxos bool,
	hashedEthereumAddress string,
) (map[string]*big.Int, error) {
	shieldedPrivateKey, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	shieldedBig, err := utils.ParseBigInt(shieldedPrivateKey)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 32)
	shieldedBig.FillBytes(data)

	keyCiphertext, inputCiphertext, err := MakeHandshakeAndEncrypt(ctx, data)
	if err != nil {
		return nil, err
	}

	resp, err := api.GetBalancesEnclaveCall(ctx, chainID, keyCiphertext, inputCiphertext, useBlockedUtxos, hashedEthereumAddress)
	if err != nil {
		return nil, err
	}

	balances := make(map[string]*big.Int, len(resp.PreConfirmedBalances))
	for _, entry := range resp.PreConfirmedBalances {
		tokenKey, err := normalizeEnclaveBalanceTokenAddress(chainID, entry.Erc20TokenAddress)
		if err != nil {
			return nil, err
		}
		amount, err := utils.ParseBigInt(entry.Amount)
		if err != nil {
			return nil, err
		}
		balances[tokenKey] = amount
	}
	return balances, nil
}

func normalizeEnclaveBalanceTokenAddress(chainID int, enclaveTokenAddress string) (string, error) {
	if constants.IsSolanaLike(chainID) {
		_, base58Mint, err := normalizeSolanaMint(enclaveTokenAddress)
		if err != nil {
			return "", err
		}
		return strings.ToLower(base58Mint), nil
	}
	if strings.HasPrefix(enclaveTokenAddress, "0x") || strings.HasPrefix(enclaveTokenAddress, "0X") {
		return strings.ToLower(enclaveTokenAddress), nil
	}
	n, ok := new(big.Int).SetString(enclaveTokenAddress, 10)
	if !ok {
		return "", fmt.Errorf("enclave: invalid balance token address %q", enclaveTokenAddress)
	}
	return strings.ToLower(common.BigToAddress(n).Hex()), nil
}
