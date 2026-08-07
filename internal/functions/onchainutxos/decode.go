package onchainutxos

import (
	"strings"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/contractabi"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	solanadata "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/balance"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func DecodeFromReceipt(
	receipt *ethtypes.Receipt,
	userKeys *cryptokeys.UserKeys,
	chainID int,
	tokenAddress string,
) ([]*utxo.Utxo, error) {
	hinkalABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	hinkalAddress, err := constants.HinkalAddress(chainID)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	deposited := make([]*utxo.Utxo, 0)
	for _, rawLog := range receipt.Logs {
		if rawLog == nil || !strings.EqualFold(rawLog.Address.Hex(), hinkalAddress) {
			continue
		}
		event, err := blockchainevent.NewFromLog(*rawLog, hinkalABI)
		if err != nil || event.EventName != "NewCommitment" {
			continue
		}
		indexArg, err := event.GetArg("index")
		if err != nil {
			continue
		}
		index, err := utils.ParseBigInt(indexArg)
		if err != nil || index.Sign() >= 0 {
			continue
		}
		encryptedOutput, err := event.GetArg("encryptedOutput")
		if err != nil {
			continue
		}
		decoded, err := balance.DecodeUtxo(encryptedOutput, userKeys, chainID)
		if err != nil || !strings.EqualFold(decoded.Erc20TokenAddress, tokenAddress) {
			continue
		}
		commitment, err := decoded.GetCommitment()
		if err != nil {
			continue
		}
		if _, ok := seen[commitment]; ok {
			continue
		}
		seen[commitment] = struct{}{}
		deposited = append(deposited, decoded)
	}
	return deposited, nil
}

func DecodeSolanaFromTransaction(
	tx *solanadata.Transaction,
	userKeys *cryptokeys.UserKeys,
	compressedMintAddress string,
) ([]*utxo.Utxo, error) {
	if tx == nil || tx.Meta == nil {
		return nil, nil
	}

	seen := map[string]struct{}{}
	deposited := make([]*utxo.Utxo, 0)
	for _, event := range solanautils.TransactionEvents(tx.Meta) {
		if event.Name != "NewCommitment" {
			continue
		}
		encryptedOutput, ok := solanautils.IntSliceArg(event.Args["encrypted_output"])
		if !ok || len(encryptedOutput) != 0 {
			continue
		}
		onChainData, ok := solanautils.IntMatrixArg(event.Args["on_chain_data"])
		if !ok {
			continue
		}
		encodedOutput, err := utils.EncodeSolanaOnChainUtxo(utils.IntMatrixToByteMatrix(onChainData))
		if err != nil {
			continue
		}
		decoded, err := balance.DecodeSolanaOnChainUtxo(encodedOutput, userKeys)
		if err != nil || !strings.EqualFold(decoded.Erc20TokenAddress, compressedMintAddress) {
			continue
		}
		commitment, err := decoded.GetCommitment()
		if err != nil {
			continue
		}
		if _, ok := seen[commitment]; ok {
			continue
		}
		seen[commitment] = struct{}{}
		deposited = append(deposited, decoded)
	}
	return deposited, nil
}
