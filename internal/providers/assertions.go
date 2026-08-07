package providers

import (
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	_ types.IProviderAdapter = (*EthersProviderAdapter)(nil)
	_ types.IProviderAdapter = (*SolanaProviderAdapter)(nil)
	_ types.IProviderAdapter = (*TronProviderAdapter)(nil)
)
