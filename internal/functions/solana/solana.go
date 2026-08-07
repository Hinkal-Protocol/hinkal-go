package solanautils

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	solana "github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type FormattedMintAddress struct {
	MintAccountPart1  *big.Int
	MintAccountPart2  *big.Int
	CompressedAddress string
}

type HinkalInstruction struct {
	AccountIndexes []byte
	Data           []byte
	ProgramIndex   int
}

func EncodeToByte32Array(value *big.Int) [32]byte {
	var out [32]byte
	if value == nil {
		return out
	}
	value.FillBytes(out[:])
	return out
}

func bigToBE(value *big.Int, length int) []byte {
	out := make([]byte, length)
	if value != nil {
		value.FillBytes(out)
	}
	return out
}

func appendBE(dst []byte, value, length int) []byte {
	b := make([]byte, length)
	new(big.Int).SetInt64(int64(value)).FillBytes(b)
	return append(dst, b...)
}

func FormatMintAddress(mint string) (FormattedMintAddress, error) {
	keyBytes, err := base58.Decode(mint)
	if err != nil {
		return FormattedMintAddress{}, fmt.Errorf("formatMintAddress: decode %q: %w", mint, err)
	}
	mintBigInt := new(big.Int).SetBytes(keyBytes)
	divisor := new(big.Int).Lsh(big.NewInt(1), 128)
	part1 := new(big.Int).Div(mintBigInt, divisor)
	part2 := new(big.Int).Mod(mintBigInt, divisor)
	compressed, err := crypto.PoseidonBig(part1, part2)
	if err != nil {
		return FormattedMintAddress{}, err
	}
	return FormattedMintAddress{
		MintAccountPart1:  part1,
		MintAccountPart2:  part2,
		CompressedAddress: utils.ToBeHex(compressed),
	}, nil
}

func GetSolanaCalldataHash(
	dimensions types.DimDataType,
	recipient, signer solana.PublicKey,
	encryptedOutputs [][]byte,
	relayerFee, variableRate *big.Int,
	instructions []HinkalInstruction,
	remainingAccounts []solana.AccountMeta,
) *big.Int {
	bytes := []byte{
		byte(dimensions.TokenNumber),
		byte(dimensions.NullifierAmount),
		byte(dimensions.OutputAmount),
	}
	bytes = append(bytes, recipient.Bytes()...)
	bytes = append(bytes, signer.Bytes()...)

	for _, cur := range encryptedOutputs {
		bytes = appendBE(bytes, len(cur), 8)
		bytes = append(bytes, cur...)
	}

	bytes = append(bytes, bigToBE(relayerFee, 8)...)
	bytes = append(bytes, bigToBE(variableRate, 8)...)

	for _, ins := range instructions {
		bytes = appendBE(bytes, ins.ProgramIndex, 1)
		bytes = appendBE(bytes, len(ins.AccountIndexes), 8)
		bytes = append(bytes, ins.AccountIndexes...)
		bytes = appendBE(bytes, len(ins.Data), 8)
		bytes = append(bytes, ins.Data...)
	}

	for _, acc := range remainingAccounts {
		if acc.IsWritable {
			bytes = append(bytes, 1)
		} else {
			bytes = append(bytes, 0)
		}
		bytes = append(bytes, acc.PublicKey.Bytes()...)
	}

	digest := sha256.Sum256(bytes)
	hash := new(big.Int).SetBytes(digest[:])
	return hash.Mod(hash, crypto.FieldP)
}
