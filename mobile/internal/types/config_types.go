package types

type ClientConfigJSON struct {
	GenerateProofRemotely               *bool             `json:"generateProofRemotely"`
	DisableMerkleTreeUpdates            bool              `json:"disableMerkleTreeUpdates"`
	CacheFilePath                       string            `json:"cacheFilePath"`
	UseFileCache                        bool              `json:"useFileCache"`
	DisableCaching                      bool              `json:"disableCaching"`
	SerializedCache                     map[string]string `json:"serializedCache"`
	TronChainOverride                   int               `json:"tronChainOverride"`
	AllowParallelBalanceLocalDecryption bool              `json:"allowParallelBalanceLocalDecryption"`
}
