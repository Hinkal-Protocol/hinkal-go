package codec

import (
	"encoding/json"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func DecodeClientConfig(configJSON string) (*types.HinkalConfig, error) {
	var raw mobiletypes.ClientConfigJSON
	if err := json.Unmarshal([]byte(configJSON), &raw); err != nil {
		return nil, mobileerrors.InvalidJSON("configJSON", err)
	}
	return &types.HinkalConfig{
		GenerateProofRemotely:               raw.GenerateProofRemotely,
		DisableMerkleTreeUpdates:            raw.DisableMerkleTreeUpdates,
		CacheFilePath:                       raw.CacheFilePath,
		UseFileCache:                        raw.UseFileCache,
		DisableCaching:                      raw.DisableCaching,
		SerializedCache:                     raw.SerializedCache,
		TronChainOverride:                   raw.TronChainOverride,
		AllowParallelBalanceLocalDecryption: raw.AllowParallelBalanceLocalDecryption,
	}, nil
}
