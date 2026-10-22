package receivevault

import (
	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type receiveVaultClaim struct {
	userKeys      *cryptokeys.UserKeys
	network       types.ReceiveVaultNetwork
	encryptionKey string
	accountKey    string
}

type ReceiveVaultSettleFeeParams struct {
	ExternalActionID        types.ExternalActionID
	SolanaTransactionParams *api.SolanaGasEstimateParams
}
