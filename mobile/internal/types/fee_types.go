package types

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type FeeStructureArgs struct {
	ChainID      int
	TokenAddrs   []string
	Calls        []types.CallInfo
	VariableRate *big.Int
	SolanaParams *api.SolanaGasEstimateParams
}
