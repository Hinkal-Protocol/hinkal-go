package types

type HandshakeResponseType struct {
	PublicKey string `json:"public_key"`
}

type RemoteProofType struct {
	PiA      []string   `json:"pi_a"`
	PiB      [][]string `json:"pi_b"`
	PiC      []string   `json:"pi_c"`
	Protocol string     `json:"protocol"`
}

type GenerateProofResponseType struct {
	Proof         RemoteProofType   `json:"proof"`
	PublicSignals []string          `json:"public_signals"`
	ZkCalldata    NewZkCallDataType `json:"zk_calldata"`
}
