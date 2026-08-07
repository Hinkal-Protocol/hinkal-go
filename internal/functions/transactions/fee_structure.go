package transactions

import (
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func normalizeFeeStructure(feeStructure types.FeeStructure) types.FeeStructure {
	if feeStructure.FeeToken == "" {
		feeStructure.FeeToken = constants.DefaultFeeToken
	}
	if feeStructure.FlatFee == nil {
		feeStructure.FlatFee = big.NewInt(0)
	}
	if feeStructure.VariableRate == nil {
		feeStructure.VariableRate = big.NewInt(0)
	}
	return feeStructure
}
