package hinkal

import (
	"context"
	"encoding/json"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	solanadata "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/onchainutxos"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func FetchClaimableUtxos(h *core.Hinkal, claimable map[string]*utxo.Utxo, chainID int64, ethAddress, signature string, isSolanaLedger bool, txMessageForSolanaLedger string) (string, error) {
	items, err := h.GetUtxosFromEnclave(
		context.Background(),
		ethAddress,
		signature,
		int(chainID),
		isSolanaLedger,
		txMessageForSolanaLedger,
	)
	if err != nil {
		return "", err
	}

	clear(claimable)
	out := make([]mobiletypes.ClaimableUtxoJSON, 0, len(items))
	for _, item := range items {
		u, err := claimableFromEnclaveItem(item)
		if err != nil {
			return "", mobileerrors.Wrap("map enclave item", err)
		}
		entry, err := registerClaimable(claimable, u)
		if err != nil {
			return "", err
		}
		entry.SenderAddress = item.SenderAddress
		out = append(out, entry)
	}
	return codec.JSONString(out)
}

func registerDecodedUtxos(claimable map[string]*utxo.Utxo, utxos []*utxo.Utxo) (string, error) {
	out := make([]mobiletypes.ClaimableUtxoJSON, 0, len(utxos))
	for _, u := range utxos {
		entry, err := registerClaimable(claimable, u)
		if err != nil {
			return "", err
		}
		out = append(out, entry)
	}
	return codec.JSONString(out)
}

func DecodeUtxosFromReceipt(claimable map[string]*utxo.Utxo, chainID int64, receiptJSON, userKeysSignature, tokenAddr string) (string, error) {
	userKeys := codec.DecodeUserKeys(userKeysSignature)
	if userKeys == nil {
		return "", mobileerrors.ErrUserKeysSignatureRequired
	}
	var receipt ethtypes.Receipt
	if err := json.Unmarshal([]byte(receiptJSON), &receipt); err != nil {
		return "", mobileerrors.InvalidJSON("receiptJSON", err)
	}
	utxos, err := onchainutxos.DecodeFromReceipt(&receipt, userKeys, int(chainID), tokenAddr)
	if err != nil {
		return "", err
	}
	return registerDecodedUtxos(claimable, utxos)
}

func DecodeSolanaUtxosFromTransaction(claimable map[string]*utxo.Utxo, transactionJSON, userKeysSignature, compressedMintAddress string) (string, error) {
	userKeys := codec.DecodeUserKeys(userKeysSignature)
	if userKeys == nil {
		return "", mobileerrors.ErrUserKeysSignatureRequired
	}
	var tx solanadata.Transaction
	if err := json.Unmarshal([]byte(transactionJSON), &tx); err != nil {
		return "", mobileerrors.InvalidJSON("transactionJSON", err)
	}
	utxos, err := onchainutxos.DecodeSolanaFromTransaction(&tx, userKeys, compressedMintAddress)
	if err != nil {
		return "", err
	}
	return registerDecodedUtxos(claimable, utxos)
}

func StoreUtxoInEnclave(h *core.Hinkal, claimable map[string]*utxo.Utxo, chainID int64, senderAddress, recipientEthAddress, handle, claimableSignature string) error {
	u, err := claimableByHandle(claimable, handle)
	if err != nil {
		return err
	}
	return h.StoreUtxoInEnclave(
		context.Background(),
		senderAddress,
		recipientEthAddress,
		u,
		int(chainID),
		claimableSignature,
	)
}

func claimableFromEnclaveItem(item types.UtxoConstructorParamsWithSenderAddress) (*utxo.Utxo, error) {
	params := item.ResolvedUtxoParams()
	if params.NullifyingKey == "" && item.ClaimableSignature != "" {
		nullifyingKey, err := cryptokeys.NewUserKeys(item.ClaimableSignature).GetShieldedPrivateKey()
		if err != nil {
			return nil, err
		}
		params.NullifyingKey = nullifyingKey
	}
	return utxo.NewUtxo(params)
}

func claimableByHandle(claimable map[string]*utxo.Utxo, handle string) (*utxo.Utxo, error) {
	u, ok := claimable[handle]
	if !ok {
		return nil, mobileerrors.ErrUnknownClaimableHandle
	}
	return u, nil
}

func registerClaimable(claimable map[string]*utxo.Utxo, u *utxo.Utxo) (mobiletypes.ClaimableUtxoJSON, error) {
	handle, err := u.GetCommitment()
	if err != nil {
		return mobiletypes.ClaimableUtxoJSON{}, mobileerrors.Wrap("commitment", err)
	}
	claimable[handle] = u
	return mobiletypes.ClaimableUtxoJSON{
		Handle:       handle,
		Amount:       codec.EncodeBig(u.Amount),
		TokenAddress: u.Erc20TokenAddress,
		Timestamp:    u.TimeStamp,
	}, nil
}
