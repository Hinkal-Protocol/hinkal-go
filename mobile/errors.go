package mobile

import (
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
)

const (
	ErrCodeInsufficientFundsToTransact = errorhandling.ErrCodeInsufficientFundsToTransact
	ErrCodeDecimalsLimit               = errorhandling.ErrCodeDecimalsLimit
	ErrCodeUtxoLimitations             = errorhandling.ErrCodeUtxoLimitations
	ErrCodeFeesOverTransactionValue    = errorhandling.ErrCodeFeesOverTransactionValue
	ErrCodeChainAddressChanged         = errorhandling.ErrCodeChainAddressChanged
)
