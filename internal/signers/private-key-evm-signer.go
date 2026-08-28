package signers

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type PrivateKeyEVMSigner struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

func NewPrivateKeyEVMSigner(privateKeyHex string) (*PrivateKeyEVMSigner, error) {
	hex := strings.TrimPrefix(privateKeyHex, "0x")
	key, err := crypto.HexToECDSA(hex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return &PrivateKeyEVMSigner{
		key:     key,
		address: crypto.PubkeyToAddress(key.PublicKey),
	}, nil
}

func (s *PrivateKeyEVMSigner) GetAddress(_ context.Context) (common.Address, error) {
	return s.address, nil
}

func (s *PrivateKeyEVMSigner) SignMessage(_ context.Context, message string) ([]byte, error) {
	hash := accounts.TextHash([]byte(message))
	sig, err := crypto.Sign(hash, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	// crypto.Sign returns V as 0 or 1 (raw ECDSA recovery id).
	// Ethereum personal_sign convention (inherited from Bitcoin) uses 27 or 28.
	sig[64] += 27
	return sig, nil
}

func (s *PrivateKeyEVMSigner) SignTypedData(_ context.Context, typedDataHash []byte) ([]byte, error) {
	sig, err := crypto.Sign(typedDataHash, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign typed data: %w", err)
	}
	sig[64] += 27
	return sig, nil
}

func (s *PrivateKeyEVMSigner) SignTransaction(_ context.Context, tx *ethtypes.Transaction) (*ethtypes.Transaction, error) {
	signer := ethtypes.LatestSignerForChainID(tx.ChainId())
	signed, err := ethtypes.SignTx(tx, signer, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	return signed, nil
}

func (s *PrivateKeyEVMSigner) BroadcastTransaction(ctx context.Context, tx *ethtypes.Transaction) (string, error) {
	chainID := int(tx.ChainId().Int64())
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return "", err
	}
	client, err := api.DialEthClientWithFallback(chainID, rpcURL)
	if err != nil {
		return "", fmt.Errorf("dial rpc: %w", err)
	}
	defer client.Close()
	if err := client.SendTransaction(ctx, tx); err != nil {
		return "", fmt.Errorf("broadcast transaction: %w", err)
	}
	return tx.Hash().Hex(), nil
}
