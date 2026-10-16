package transactions

import (
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func toBase58IfTron(chainID int, addr string) (string, error) {
	if !constants.IsTronLike(chainID) || !constants.IsTronHexAddress(addr) {
		return addr, nil
	}
	return utils.EVMHexToTronBase58Address(addr)
}
