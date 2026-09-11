package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// ReceiveVaultParams mirrors receive-vault.types.ts. Every field is part of the vault's CREATE2
// salt, so the relayer cannot change one and still point at an address that holds anything.
type ReceiveVaultParams struct {
	Recipient StealthAddressStructure
	Owner     string
	Version   *big.Int
}

// ReceiveVaultParamsJSON is the wire form of ReceiveVaultParams, with the big values as strings.
type ReceiveVaultParamsJSON struct {
	Recipient StealthAddressStructureJSON `json:"recipient"`
	Owner     string                      `json:"owner"`
	Version   string                      `json:"version"`
}

type ReceiveVaultNetwork string

const (
	ReceiveVaultNetworkEvm    ReceiveVaultNetwork = "evm"
	ReceiveVaultNetworkTron   ReceiveVaultNetwork = "tron"
	ReceiveVaultNetworkSolana ReceiveVaultNetwork = "solana"
)

// ReceiveVaultRecord holds the terms an address was built with. Version IS part of the salt (see
// ReceiveVaultParams) - it is fixed at creation and must be read back exactly as stored, never
// recomputed as "whatever is current now".
type ReceiveVaultRecord struct {
	Index        int                 `json:"index"`
	Network      ReceiveVaultNetwork `json:"network"`
	Version      string              `json:"version"`
	VaultAddress string              `json:"vaultAddress"`
}

type ReceiveVaultOutputSignature struct {
	ChainID   int    `json:"chainId"`
	Signature string `json:"signature"`
}

type ReceiveVaultBlockReason string

const (
	ReceiveVaultBlockNotShieldable ReceiveVaultBlockReason = "notShieldable"
	ReceiveVaultBlockRiskySender   ReceiveVaultBlockReason = "riskySender"
)

// ReceiveVaultBlock flags a token the relayer refused to shield: the one thing about a deposit
// address its owner cannot work out from chain state. The balance itself is read on chain.
type ReceiveVaultBlock struct {
	VaultAddress string `json:"vaultAddress"`
	ChainID      int    `json:"chainId"`
	Token        string `json:"token"`
}

// ReceiveVaultAccountProof proves control of the shielded account itself rather than of one deposit
// address, so the terms needed to re-derive an address can be fetched back after local storage is gone.
type ReceiveVaultAccountProof struct {
	AccountKey string `json:"accountKey"`
	Deadline   string `json:"deadline"`
	Signature  string `json:"signature"`
}

// ReceiveVaultBlockedFund is a balance the relayer declined to shield, sitting in a vault only its
// owner can derive.
type ReceiveVaultBlockedFund struct {
	Record ReceiveVaultRecord
	Token  ERC20Token
	Amount *big.Int
	Reason ReceiveVaultBlockReason
}

// ReceiveVaultAccount.Entries are deposit addresses the user has handed out, accepting any
// supported token.
type ReceiveVaultAccount struct {
	Entries      []ReceiveVaultRecord
	BlockedFunds []ReceiveVaultBlockedFund
}

// ReceiveVaultTerms is a ReceiveVaultRecord without the address it derives, the counterpart of
// receive-vault.utils.ts's Omit<ReceiveVaultRecord, 'vaultAddress'>.
type ReceiveVaultTerms struct {
	Index   int
	Network ReceiveVaultNetwork
	Version string
}

// ReceiveVaultRecipientTuple is the recipient half of the params tuple the factory ABI takes.
type ReceiveVaultRecipientTuple struct {
	H0x            *big.Int
	H0y            *big.Int
	H1x            *big.Int
	H1y            *big.Int
	StealthAddress *big.Int
}

// ReceiveVaultParamsTuple is the params tuple the factory ABI takes.
type ReceiveVaultParamsTuple struct {
	Recipient ReceiveVaultRecipientTuple
	Owner     common.Address
	Version   *big.Int
}
