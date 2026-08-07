package providers

import (
	"context"
	"encoding/base64"
	"fmt"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type SolanaProgram struct {
	ProgramID solana.PublicKey
	Client    *rpc.Client
	Signer    types.SolanaSigner
}

func (p *SolanaProgram) SignAndSend(ctx context.Context, tx *solana.Transaction) (solana.Signature, error) {
	signed, err := p.Signer.SignTransaction(ctx, tx)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("sign transaction: %w", err)
	}
	sig, err := p.Client.SendTransactionWithOpts(ctx, signed, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("send transaction: %w", err)
	}
	return sig, nil
}

func (p *SolanaProgram) SignOnly(ctx context.Context, tx *solana.Transaction) (string, string, error) {
	signed, err := p.Signer.SignTransaction(ctx, tx)
	if err != nil {
		return "", "", fmt.Errorf("sign transaction: %w", err)
	}
	msg, err := signed.Message.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("serialize message: %w", err)
	}
	pubKey, err := p.Signer.GetPublicKey(ctx)
	if err != nil {
		return "", "", fmt.Errorf("get public key: %w", err)
	}
	for i, key := range signed.Message.AccountKeys {
		if key.Equals(pubKey) && i < len(signed.Signatures) {
			sig := signed.Signatures[i]
			signature, err := hexSignature(sig[:])
			if err != nil {
				return "", "", err
			}
			return signature, base64.StdEncoding.EncodeToString(msg), nil
		}
	}
	return "", "", fmt.Errorf("signature not found for public key %s", pubKey)
}
