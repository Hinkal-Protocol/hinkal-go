package utxo

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type Utxo struct {
	Amount            *big.Int
	Erc20TokenAddress string
	MintAddress       string
	TimeStamp         string
	NullifyingKey     string
	SpendingPublicKey []*big.Int
	H0                *types.JubPoint
	StealthAddress    string
	EncryptionKey     string
	IsBlocked         bool

	commitment string
	nullifier  string
}

func NewUtxo(params types.UtxoParams) (*Utxo, error) {
	timeStamp := params.TimeStamp
	if timeStamp == "" {
		timeStamp = strconv.FormatInt(utils.GetCurrentTimeInSeconds(), 10)
	}

	h0 := params.H0
	if h0 == nil {
		extraRandomization := params.Randomization
		if extraRandomization == nil && params.NullifyingKey != "" {
			seed, err := utils.RandomBigInt(31)
			if err != nil {
				return nil, err
			}
			extraRandomization, err = cryptokeys.FindCorrectRandomization(seed, params.NullifyingKey)
			if err != nil {
				return nil, err
			}
		}
		if extraRandomization != nil {
			found, err := cryptokeys.FindH0(extraRandomization, params.NullifyingKey)
			if err != nil {
				return nil, err
			}
			h0 = found
		}
		if h0 == nil {
			return nil, errors.New("UTXO requires H0 or nullifyingKey")
		}
	}

	return &Utxo{
		Amount:            params.Amount,
		Erc20TokenAddress: params.Erc20TokenAddress,
		MintAddress:       params.MintAddress,
		TimeStamp:         timeStamp,
		NullifyingKey:     params.NullifyingKey,
		SpendingPublicKey: params.SpendingPublicKey,
		H0:                h0,
		StealthAddress:    params.StealthAddress,
		EncryptionKey:     params.EncryptionKey,
		IsBlocked:         params.IsBlocked,
		commitment:        params.Commitment,
		nullifier:         params.Nullifier,
	}, nil
}

// CreateFrom builds a new Utxo from an existing one, clearing the cached commitment/nullifier and
// applying the non-zero fields of patch on top of the source's constructable parameters.
func CreateFrom(src *Utxo, patch types.UtxoParams) (*Utxo, error) {
	params := src.GetConstructableParams()
	params.Commitment = ""
	params.Nullifier = ""
	applyPatch(&params, patch)
	return NewUtxo(params)
}

func applyPatch(dst *types.UtxoParams, patch types.UtxoParams) {
	if patch.Amount != nil {
		dst.Amount = patch.Amount
	}
	if patch.Erc20TokenAddress != "" {
		dst.Erc20TokenAddress = patch.Erc20TokenAddress
	}
	if patch.MintAddress != "" {
		dst.MintAddress = patch.MintAddress
	}
	if patch.TimeStamp != "" {
		dst.TimeStamp = patch.TimeStamp
	}
	if patch.NullifyingKey != "" {
		dst.NullifyingKey = patch.NullifyingKey
	}
	if patch.SpendingPublicKey != nil {
		dst.SpendingPublicKey = patch.SpendingPublicKey
	}
	if patch.Randomization != nil {
		dst.Randomization = patch.Randomization
	}
	if patch.H0 != nil {
		dst.H0 = patch.H0
	}
	if patch.StealthAddress != "" {
		dst.StealthAddress = patch.StealthAddress
	}
	if patch.EncryptionKey != "" {
		dst.EncryptionKey = patch.EncryptionKey
	}
	if patch.Commitment != "" {
		dst.Commitment = patch.Commitment
	}
	if patch.Nullifier != "" {
		dst.Nullifier = patch.Nullifier
	}
}

func (u *Utxo) GetConstructableParams() types.UtxoParams {
	return types.UtxoParams{
		Amount:            u.Amount,
		Erc20TokenAddress: u.Erc20TokenAddress,
		MintAddress:       u.MintAddress,
		TimeStamp:         u.TimeStamp,
		NullifyingKey:     u.NullifyingKey,
		SpendingPublicKey: u.SpendingPublicKey,
		H0:                u.H0,
		StealthAddress:    u.StealthAddress,
		EncryptionKey:     u.EncryptionKey,
		Commitment:        u.commitment,
		Nullifier:         u.nullifier,
		IsBlocked:         u.IsBlocked,
	}
}

func (u *Utxo) GetBasicUtxoParams() types.UtxoParams {
	return types.UtxoParams{
		Amount:            u.Amount,
		Erc20TokenAddress: u.Erc20TokenAddress,
		MintAddress:       u.MintAddress,
		TimeStamp:         u.TimeStamp,
		H0:                u.H0,
		StealthAddress:    u.StealthAddress,
		Commitment:        u.commitment,
		Nullifier:         u.nullifier,
		IsBlocked:         u.IsBlocked,
	}
}

// GetCommitment returns the commitment hash of the UTXO, computing and caching it on first use.
func (u *Utxo) GetCommitment() (string, error) {
	if u.commitment != "" {
		return u.commitment, nil
	}
	stealth, err := u.GetStealthAddress()
	if err != nil {
		return "", err
	}
	token, err := utils.ParseBigInt(u.Erc20TokenAddress)
	if err != nil {
		return "", err
	}
	stealthBig, err := utils.ParseBigInt(stealth)
	if err != nil {
		return "", err
	}
	ts, err := utils.ParseBigInt(u.TimeStamp)
	if err != nil {
		return "", err
	}
	h, err := crypto.PoseidonBig(u.Amount, token, stealthBig, ts)
	if err != nil {
		return "", err
	}
	u.commitment = utils.ToBeHex(h)
	return u.commitment, nil
}

// GetNullifier returns the nullifier hash of the UTXO, computing and caching it on first use.
func (u *Utxo) GetNullifier() (string, error) {
	if u.nullifier != "" {
		return u.nullifier, nil
	}
	if u.NullifyingKey == "" {
		return "", errors.New("no nullifiers if private key is not provided")
	}
	commitment, err := u.GetCommitment()
	if err != nil {
		return "", err
	}
	commitmentBig, err := utils.ParseBigInt(commitment)
	if err != nil {
		return "", err
	}
	nk, err := utils.ParseBigInt(u.NullifyingKey)
	if err != nil {
		return "", err
	}
	signature, err := crypto.PoseidonBig(nk, commitmentBig)
	if err != nil {
		return "", err
	}
	nul, err := crypto.PoseidonBig(commitmentBig, signature)
	if err != nil {
		return "", err
	}
	u.nullifier = utils.ToBeHex(nul)
	return u.nullifier, nil
}

// GetStealthAddress returns the stealth address, computing it from the H0 point when it is not
// already known.
func (u *Utxo) GetStealthAddress() (string, error) {
	if u.StealthAddress != "" {
		return u.StealthAddress, nil
	}
	if u.NullifyingKey == "" {
		return "", errors.New("no stealth address in UTXO if private key is not provided")
	}
	if u.H0 == nil {
		return "", errors.New("no H0 point provided")
	}
	addr, err := cryptokeys.GetStealthAddress(*u.H0, u.NullifyingKey, u.SpendingPublicKey)
	if err != nil {
		return "", err
	}
	u.StealthAddress = addr
	return u.StealthAddress, nil
}

// GetTokenAddress returns the mint address on Solana-like chains and the ERC-20 address otherwise.
func (u *Utxo) GetTokenAddress(chainID int) (string, error) {
	tokenAddress := u.Erc20TokenAddress
	if constants.IsSolanaLike(chainID) {
		tokenAddress = u.MintAddress
	}
	if tokenAddress == "" {
		return "", errors.New("no token address provided")
	}
	return tokenAddress, nil
}

// GetEncryptionKey returns the public encryption key derived from the nullifying key, or the
// explicitly provided encryption key when no nullifying key is known.
func (u *Utxo) GetEncryptionKey() (string, error) {
	if u.NullifyingKey == "" {
		if u.EncryptionKey == "" {
			return "", errors.New("no encryption key provided in UTXO")
		}
		return u.EncryptionKey, nil
	}
	pair, err := cryptokeys.GetEncryptionKeyPair(u.NullifyingKey)
	if err != nil {
		return "", err
	}
	return pair.PublicKey, nil
}
