package balance

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/crypto"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func DecryptUtxo(encryptedData []byte, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	sk, pk, err := cryptokeys.EncryptionKeyPair(spk)
	if err != nil {
		return nil, err
	}

	plain, err := cryptokeys.DecryptSealedKeys(encryptedData, &pk, &sk, 0)
	if err != nil {
		return nil, err
	}

	// layout: amount, erc20TokenAddress, stealthAddress, timeStamp, H0x, H0y, encryptionKey[, mint]
	parts := splitHexParts(string(plain))
	if len(parts) < 7 {
		return nil, errors.New("decrypt utxo: too few fields")
	}

	amount, err := utils.ParseBigInt("0x" + parts[0])
	if err != nil {
		return nil, err
	}
	ts, err := utils.ParseBigInt("0x" + parts[3])
	if err != nil {
		return nil, err
	}
	h0x, err := utils.ParseBigInt("0x" + parts[4])
	if err != nil {
		return nil, err
	}
	h0y, err := utils.ParseBigInt("0x" + parts[5])
	if err != nil {
		return nil, err
	}

	mint := ""
	if len(parts) > 7 {
		mintBytes, err := hex.DecodeString(parts[7])
		if err == nil && len(mintBytes) == 32 {
			mint = base58.Encode(mintBytes)
		}
	}

	return utxo.NewUtxo(types.UtxoParams{
		Amount:            amount,
		Erc20TokenAddress: "0x" + parts[1],
		MintAddress:       mint,
		TimeStamp:         ts.String(),
		NullifyingKey:     spk,
		StealthAddress:    "0x" + parts[2],
		H0:                &types.JubPoint{h0x, h0y},
	})
}

func DecryptUtxoHex(encryptedHex string, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	return DecryptUtxo(common.FromHex(encryptedHex), uk)
}

func DecodeUtxo(encodedOutput string, uk *cryptokeys.UserKeys, chainID int) (*utxo.Utxo, error) {
	if constants.IsSolanaLike(chainID) {
		return DecodeSolanaOnChainUtxo(encodedOutput, uk)
	}
	return DecodeEvmUtxoHex(encodedOutput, uk)
}

type onChainStealthAddressStructureTuple struct {
	H0x            *big.Int
	H0y            *big.Int
	H1x            *big.Int
	H1y            *big.Int
	StealthAddress *big.Int
}

type onChainUtxoTuple struct {
	Amount                  *big.Int
	Erc20Address            common.Address
	StealthAddressStructure onChainStealthAddressStructureTuple
	TimeStamp               *big.Int
}

var onChainUtxoArgs = func() abi.Arguments {
	stealthComponents := []abi.ArgumentMarshaling{
		{Name: "h0x", Type: "uint256"},
		{Name: "h0y", Type: "uint256"},
		{Name: "h1x", Type: "uint256"},
		{Name: "h1y", Type: "uint256"},
		{Name: "stealthAddress", Type: "uint256"},
	}
	utxoType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "amount", Type: "uint256"},
		{Name: "erc20Address", Type: "address"},
		{Name: "stealthAddressStructure", Type: "tuple", Components: stealthComponents},
		{Name: "timeStamp", Type: "uint256"},
	})
	if err != nil {
		panic(fmt.Sprintf("balance: invalid on-chain utxo abi: %v", err))
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		panic(fmt.Sprintf("balance: invalid bytes abi type: %v", err))
	}
	return abi.Arguments{{Type: utxoType}, {Type: bytesType}}
}()

func DecodeEvmUtxo(encodedData []byte, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	values, err := onChainUtxoArgs.Unpack(encodedData)
	if err != nil || len(values) != 2 {
		return nil, fmt.Errorf("decode evm utxo: %w", err)
	}
	converted, convOk := abi.ConvertType(values[0], new(onChainUtxoTuple)).(*onChainUtxoTuple)
	if !convOk {
		return nil, errors.New("decode evm utxo: unexpected abi-decoded type")
	}
	decoded := *converted

	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}

	s := decoded.StealthAddressStructure
	ok, err := checkUtxoSignature(s.H0x, s.H0y, s.H1y, spk)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("UTXO doesn't belong to user")
	}

	return utxo.NewUtxo(types.UtxoParams{
		Amount:            decoded.Amount,
		Erc20TokenAddress: decoded.Erc20Address.Hex(),
		TimeStamp:         decoded.TimeStamp.String(),
		NullifyingKey:     spk,
		StealthAddress:    utils.ToBeHex(s.StealthAddress),
		H0:                &types.JubPoint{s.H0x, s.H0y},
	})
}

func DecodeEvmUtxoHex(encodedHex string, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	return DecodeEvmUtxo(common.FromHex(encodedHex), uk)
}

func DecodeSolanaOnChainUtxo(encodedOutput string, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	const prefix = "solana-on-chain-utxo:"
	if !strings.HasPrefix(encodedOutput, prefix) {
		return nil, errors.New("invalid encoded Solana UTXO payload")
	}
	encodedData := common.FromHex(strings.TrimPrefix(encodedOutput, prefix))
	if len(encodedData) != 32*9 {
		return nil, errors.New("malformed encoded Solana UTXO payload")
	}
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}

	wordBytes := func(i int) []byte {
		return encodedData[i*32 : (i+1)*32]
	}
	word := func(i int) *big.Int {
		return new(big.Int).SetBytes(wordBytes(i))
	}
	amount := word(0)
	mintPart1 := word(1)
	mintPart2 := word(2)
	h0x := word(3)
	h0y := word(4)
	// word(5) is h1x (unused here; only y-coordinates are needed for the signature check)
	h1y := word(6)
	stealthAddress := utils.ToBeHex(word(7))
	ts := word(8)

	ok, err := checkUtxoSignature(h0x, h0y, h1y, spk)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("UTXO doesn't belong to user")
	}

	mintBytes := make([]byte, 0, 32)
	mintBytes = append(mintBytes, wordBytes(1)[16:]...)
	mintBytes = append(mintBytes, wordBytes(2)[16:]...)
	tokenHash, err := crypto.PoseidonBig(mintPart1, mintPart2)
	if err != nil {
		return nil, err
	}

	return utxo.NewUtxo(types.UtxoParams{
		Amount:            amount,
		Erc20TokenAddress: utils.ToBeHex(tokenHash),
		MintAddress:       base58.Encode(mintBytes),
		TimeStamp:         ts.String(),
		NullifyingKey:     spk,
		StealthAddress:    stealthAddress,
		H0:                &types.JubPoint{h0x, h0y},
	})
}

func checkUtxoSignature(h0x, h0y, h1y *big.Int, privateKey string) (bool, error) {
	return cryptokeys.VerifyStealthPair(
		types.JubPoint{new(big.Int).Set(h0x), new(big.Int).Set(h0y)},
		types.JubPoint{new(big.Int), new(big.Int).Set(h1y)},
		privateKey,
		true,
	)
}

func splitHexParts(s string) []string {
	raw := strings.Split(s, "0x")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
