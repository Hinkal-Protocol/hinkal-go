package receivevault

import (
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type receiveVaultClaim struct {
	userKeys      *cryptokeys.UserKeys
	network       types.ReceiveVaultNetwork
	encryptionKey string
	accountKey    string
}
