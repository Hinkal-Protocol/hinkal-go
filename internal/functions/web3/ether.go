package web3

import (
	"errors"
	"math/big"

	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func GetAmountInToken(token types.ERC20Token, amount *big.Int) string {
	return utils.FormatUnits(amount, token.Decimals)
}

func GetAmountInWei(token types.ERC20Token, amount string) (*big.Int, error) {
	parsed, err := utils.ParseUnits(amount, 18)
	if err != nil {
		return nil, errors.New(errorhandling.ErrCodeDecimalsLimit)
	}
	decimalsToRemove := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(18-token.Decimals)), nil)
	return new(big.Int).Quo(parsed, decimalsToRemove), nil
}

func GetAmountWithPrecision(balance *big.Int, token types.ERC20Token, precision int) (string, error) {
	amount := utils.FormatUnits(balance, token.Decimals)
	decimalsToRemove := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(18-precision)), nil)
	parsed, err := utils.ParseUnits(amount, 18)
	if err != nil {
		return "", errors.New(errorhandling.ErrCodeDecimalsLimit)
	}
	preciseBigInt := new(big.Int).Quo(parsed, decimalsToRemove)
	return utils.FormatUnits(preciseBigInt, precision), nil
}
