package mobile

type HostWallet interface {
	Address() (string, error)
	ChainID() int64
	PersonalSign(message string) (string, error)
	SendTransaction(toHex, dataHex, valueDec string, gasLimit int64) (string, error)
	SwitchChain(chainID int64) error
}

type HostSolanaSigner interface {
	PublicKey() (string, error)
	SignMessage(message []byte) ([]byte, error)
	SignTransaction(tx []byte) ([]byte, error)
}

type HostTronSigner interface {
	Address() (string, error)
	SignMessage(message string) ([]byte, error)
	SignTxHash(txHash []byte) ([]byte, error)
}
