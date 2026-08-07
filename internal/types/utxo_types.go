package types

import (
	"encoding/json"
	"math/big"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

type UtxoParams struct {
	Amount            *big.Int
	Erc20TokenAddress string
	MintAddress       string
	TimeStamp         string
	NullifyingKey     string
	SpendingPublicKey []*big.Int
	Randomization     *big.Int
	H0                *JubPoint
	StealthAddress    string
	EncryptionKey     string
	Commitment        string
	Nullifier         string
	IsBlocked         bool
}

type UtxoConstructorParamsWithSenderAddress struct {
	UtxoParams         UtxoParams
	SenderAddress      string
	ClaimableSignature string
	ShieldedPrivateKey string
}

func (p UtxoConstructorParamsWithSenderAddress) ResolvedUtxoParams() UtxoParams {
	params := p.UtxoParams
	if params.NullifyingKey == "" && p.ShieldedPrivateKey != "" {
		params.NullifyingKey = p.ShieldedPrivateKey
	}
	return params
}

func (p UtxoParams) MarshalJSON() ([]byte, error) {
	w := struct {
		Amount            string   `json:"amount"`
		Erc20TokenAddress string   `json:"erc20TokenAddress"`
		MintAddress       string   `json:"mintAddress,omitempty"`
		TimeStamp         string   `json:"timeStamp,omitempty"`
		NullifyingKey     string   `json:"nullifyingKey,omitempty"`
		SpendingPublicKey []string `json:"spendingPublicKey,omitempty"`
		Randomization     string   `json:"randomization,omitempty"`
		H0                []string `json:"H0,omitempty"`
		StealthAddress    string   `json:"stealthAddress,omitempty"`
		EncryptionKey     string   `json:"encryptionKey,omitempty"`
		Commitment        string   `json:"commitment,omitempty"`
		Nullifier         string   `json:"nullifier,omitempty"`
		IsBlocked         bool     `json:"isBlocked,omitempty"`
	}{
		Amount:            "0",
		Erc20TokenAddress: p.Erc20TokenAddress,
		MintAddress:       p.MintAddress,
		TimeStamp:         p.TimeStamp,
		NullifyingKey:     p.NullifyingKey,
		StealthAddress:    p.StealthAddress,
		EncryptionKey:     p.EncryptionKey,
		Commitment:        p.Commitment,
		Nullifier:         p.Nullifier,
		IsBlocked:         p.IsBlocked,
	}
	if p.Amount != nil {
		w.Amount = p.Amount.String()
	}
	if len(p.SpendingPublicKey) > 0 {
		w.SpendingPublicKey = make([]string, len(p.SpendingPublicKey))
		for i, v := range p.SpendingPublicKey {
			w.SpendingPublicKey[i] = v.String()
		}
	}
	if p.Randomization != nil {
		w.Randomization = p.Randomization.String()
	}
	if p.H0 != nil {
		w.H0 = []string{p.H0[0].String(), p.H0[1].String()}
	}
	return json.Marshal(w)
}

func (p *UtxoParams) UnmarshalJSON(data []byte) error {
	var w struct {
		Amount            string   `json:"amount"`
		Erc20TokenAddress string   `json:"erc20TokenAddress"`
		MintAddress       string   `json:"mintAddress"`
		TimeStamp         string   `json:"timeStamp"`
		NullifyingKey     string   `json:"nullifyingKey"`
		SpendingPublicKey []string `json:"spendingPublicKey"`
		Randomization     string   `json:"randomization"`
		H0                []string `json:"H0"`
		StealthAddress    string   `json:"stealthAddress"`
		EncryptionKey     string   `json:"encryptionKey"`
		Commitment        string   `json:"commitment"`
		Nullifier         string   `json:"nullifier"`
		IsBlocked         bool     `json:"isBlocked"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	amount, err := optionalBigInt(w.Amount)
	if err != nil {
		return err
	}
	randomization, err := optionalBigInt(w.Randomization)
	if err != nil {
		return err
	}
	spendingPublicKey, err := bigIntSlice(w.SpendingPublicKey)
	if err != nil {
		return err
	}
	h0, err := jubPointFromStrings(w.H0)
	if err != nil {
		return err
	}

	p.Amount = amount
	p.Erc20TokenAddress = w.Erc20TokenAddress
	p.MintAddress = w.MintAddress
	p.TimeStamp = w.TimeStamp
	p.NullifyingKey = w.NullifyingKey
	p.SpendingPublicKey = spendingPublicKey
	p.Randomization = randomization
	p.H0 = h0
	p.StealthAddress = w.StealthAddress
	p.EncryptionKey = w.EncryptionKey
	p.Commitment = w.Commitment
	p.Nullifier = w.Nullifier
	p.IsBlocked = w.IsBlocked
	return nil
}

func optionalBigInt(s string) (*big.Int, error) {
	if s == "" {
		return nil, nil
	}
	return utils.ParseBigInt(s)
}

func bigIntSlice(values []string) ([]*big.Int, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]*big.Int, len(values))
	for i, v := range values {
		n, err := utils.ParseBigInt(v)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func jubPointFromStrings(values []string) (*JubPoint, error) {
	if len(values) != 2 {
		return nil, nil
	}
	x, err := utils.ParseBigInt(values[0])
	if err != nil {
		return nil, err
	}
	y, err := utils.ParseBigInt(values[1])
	if err != nil {
		return nil, err
	}
	return &JubPoint{x, y}, nil
}
