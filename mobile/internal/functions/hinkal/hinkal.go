package hinkal

import (
	"context"

	core "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal"
	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/codec"
)

func InitUserKeysWithSignature(h *core.Hinkal, signature string) {
	h.InitUserKeysWithSignature(signature)
}

func InitUserKeysFromSeedPhrases(h *core.Hinkal, seedPhrasesJSON string) error {
	seedPhrases, err := codec.DecodeStrings(seedPhrasesJSON)
	if err != nil {
		return err
	}
	return h.InitUserKeysFromSeedPhrases(seedPhrases)
}

func ResetMerkle(h *core.Hinkal, chainIDsJSON string) error {
	chainIDs, err := codec.DecodeChainIDs(chainIDsJSON)
	if err != nil {
		return err
	}
	return h.ResetMerkle(context.Background(), chainIDs...)
}

func ResetMerkleTreesIfNecessary(h *core.Hinkal, chainIDsJSON string) error {
	chainIDs, err := codec.DecodeChainIDs(chainIDsJSON)
	if err != nil {
		return err
	}
	return h.ResetMerkleTreesIfNecessary(context.Background(), chainIDs...)
}

func GetTotalBalance(h *core.Hinkal, chainID int64, userKeysSignature, ethAddress string, resetCache, useBlockedUtxos bool) (string, error) {
	balances, err := h.GetTotalBalance(
		context.Background(),
		int(chainID),
		codec.DecodeUserKeys(userKeysSignature),
		ethAddress,
		resetCache,
		useBlockedUtxos,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTokenBalances(balances)
}

func GetStuckShieldedBalances(h *core.Hinkal, chainID int64, userKeysSignature, ethAddress string) (string, error) {
	balances, err := h.GetStuckShieldedBalances(
		context.Background(),
		int(chainID),
		codec.DecodeUserKeys(userKeysSignature),
		ethAddress,
	)
	if err != nil {
		return "", err
	}
	return codec.EncodeTokenBalances(balances)
}

func GetSupportedChains(h *core.Hinkal) (string, error) {
	return codec.JSONString(h.GetSupportedChains())
}

func GetShieldedPublicKey(h *core.Hinkal) (string, error) {
	return h.GetShieldedPublicKey()
}
