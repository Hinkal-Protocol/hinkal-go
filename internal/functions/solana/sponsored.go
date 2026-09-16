package solanautils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
)

func SendSolanaSponsoredTx(ctx context.Context, signingKey string, intent api.SolanaSponsoredBuildRequestBody) (string, *big.Int, error) {
	if signingKey == "" {
		return "", nil, errors.New("solana: no signing key available for Solana sponsored transaction")
	}

	build, err := api.CallRelayerSolanaSponsoredBuildAPI(ctx, intent)
	if err != nil {
		return "", nil, err
	}
	if build.Status != "success" || build.SerializedTx == "" || build.TicketID == "" {
		return "", nil, sponsoredError(build.Error, "Solana sponsored transaction could not be built")
	}

	raw, err := base64.StdEncoding.DecodeString(build.SerializedTx)
	if err != nil {
		return "", nil, fmt.Errorf("solana: could not decode sponsored transaction: %w", err)
	}
	tx, err := solana.TransactionFromBytes(raw)
	if err != nil {
		return "", nil, fmt.Errorf("solana: could not deserialize sponsored transaction: %w", err)
	}

	privateKey, err := solana.PrivateKeyFromBase58(signingKey)
	if err != nil {
		return "", nil, fmt.Errorf("solana: invalid sponsored transaction signing key: %w", err)
	}
	publicKey := privateKey.PublicKey()
	if _, err := tx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(publicKey) {
			return &privateKey
		}
		return nil
	}); err != nil {
		return "", nil, fmt.Errorf("solana: could not sign sponsored transaction: %w", err)
	}

	signed, err := tx.MarshalBinary()
	if err != nil {
		return "", nil, fmt.Errorf("solana: could not serialize sponsored transaction: %w", err)
	}

	submit, err := api.CallRelayerSolanaSponsoredSubmitAPI(ctx, api.SolanaSponsoredSubmitRequestBody{
		ChainID:      intent.ChainID,
		TicketID:     build.TicketID,
		SerializedTx: base64.StdEncoding.EncodeToString(signed),
	})
	if err != nil {
		return "", nil, err
	}
	if submit.Status != "success" || submit.TxHash == "" {
		return "", nil, sponsoredError(submit.Error, "Solana sponsored transaction failed")
	}

	feeWei := new(big.Int)
	if build.Fee != "" {
		if _, ok := feeWei.SetString(build.Fee, 10); !ok {
			return "", nil, fmt.Errorf("solana: invalid sponsored transaction fee %q", build.Fee)
		}
	}
	return submit.TxHash, feeWei, nil
}

func sponsoredError(relayerError, fallback string) error {
	if relayerError != "" {
		return errors.New(relayerError)
	}
	return errors.New(fallback)
}
