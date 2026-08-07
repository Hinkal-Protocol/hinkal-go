package types

type MerkleTreeJSON struct {
	Tree        map[string]string `json:"tree"`
	ReverseTree map[string]string `json:"reverseTree,omitempty"`
	Count       string            `json:"count"`
	Index       string            `json:"index"`
}

type SenderAddressIndexEntry struct {
	Address string `json:"address"`
	Index   int    `json:"index"`
}

type SerializedEncryptedOutputWithSign struct {
	Value         string `json:"value"`
	IsPositive    string `json:"isPositive"`
	IsBlocked     bool   `json:"isBlocked"`
	EncryptedBlob string `json:"encryptedBlob,omitempty"`
}

type CommitmentsSerializedSnapshot struct {
	LatestBlockNumber            *uint64                             `json:"latestBlockNumber,omitempty"`
	MerkleTree                   *MerkleTreeJSON                     `json:"merkleTree,omitempty"`
	EncryptedOutputs             []SerializedEncryptedOutputWithSign `json:"encryptedOutputs,omitempty"`
	EncryptedOutputsByCommitment []EncryptedOutputWithCommitment     `json:"encryptedOutputsByCommitment,omitempty"`
}

type NullifierSerializedSnapshot struct {
	LatestBlockNumber *uint64  `json:"latestBlockNumber,omitempty"`
	Nullifiers        []string `json:"nullifiers,omitempty"`
}

type CommitmentsTreeSnapshot struct {
	MerkleTree                   MerkleTreeJSON                      `json:"merkleTree"`
	LatestBlockNumber            uint64                              `json:"latestBlockNumber"`
	EncryptedOutputs             []SerializedEncryptedOutputWithSign `json:"encryptedOutputs"`
	HinkalAddress                string                              `json:"hinkalAddress"`
	ChainID                      int                                 `json:"chainId"`
	EncryptedOutputsByCommitment []EncryptedOutputWithCommitment     `json:"encryptedOutputsByCommitment"`
}

type NullifiersSnapshot struct {
	Nullifiers        []string `json:"nullifiers"`
	LatestBlockNumber uint64   `json:"latestBlockNumber"`
}

type SnapshotsResponse struct {
	Commitments CommitmentsTreeSnapshot `json:"commitments"`
	Nullifiers  NullifiersSnapshot      `json:"nullifiers"`
}
