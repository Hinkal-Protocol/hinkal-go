package types

import "math/big"

type TemporaryWalletRecoveryDestination string

const (
	TemporaryWalletRecoveryDestinationPublic       TemporaryWalletRecoveryDestination = "public"
	TemporaryWalletRecoveryDestinationConfidential TemporaryWalletRecoveryDestination = "confidential"
)

type ERC20Token struct {
	ChainID                     int      `json:"chainId"`
	Erc20TokenAddress           string   `json:"erc20TokenAddress"`
	WrappedErc20TokenAddress    string   `json:"wrappedErc20TokenAddress,omitempty"`
	UnderlyingErc20TokenAddress string   `json:"underlyingErc20TokenAddress,omitempty"`
	Name                        string   `json:"name"`
	Symbol                      string   `json:"symbol"`
	Decimals                    int      `json:"decimals"`
	NFTTokenType                string   `json:"nftTokenType,omitempty"`
	LogoURI                     string   `json:"logoURI,omitempty"`
	LogoURIs                    []string `json:"logoURIs,omitempty"`
	Whitelisted                 bool     `json:"whitelisted,omitempty"`
	IsCustom                    bool     `json:"isCustom,omitempty"`
	TokenIDs                    []string `json:"tokenIds,omitempty"`
	ApprovalType                *int     `json:"approvalType,omitempty"`
	IsVolatile                  bool     `json:"isVolatile,omitempty"`
	SharedAddress               string   `json:"sharedAddress,omitempty"`
	IsPendleToken               bool     `json:"isPendleToken,omitempty"`
	IsHToken                    bool     `json:"isHToken,omitempty"`
	HasHToken                   bool     `json:"hasHToken,omitempty"`
	AaveToken                   bool     `json:"aaveToken,omitempty"`
	AllowanceStorageOffset      *int     `json:"allowanceStorageOffset,omitempty"`
	BalanceStorageOffset        *int     `json:"balanceStorageOffset,omitempty"`
	IsVyper                     bool     `json:"isVyper,omitempty"`
	IsSpam                      bool     `json:"isSpam,omitempty"`
	Is2022Program               bool     `json:"is2022Program,omitempty"`
}

type TokenBalance struct {
	Token     ERC20Token
	Balance   *big.Int
	Timestamp string
}

type VolatileTokenChange struct {
	TokenAddress  string `json:"tokenAddress"`
	Amount        string `json:"amount"`
	TargetAddress string `json:"targetAddress,omitempty"`
	OriginalIndex int    `json:"originalIndex"`
}
