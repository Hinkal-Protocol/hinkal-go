package types

import "math/big"

type JubPoint [2]*big.Int

type SpendingKeyPair struct {
	PrivSpendingKey     string
	PubSpendingBJJPoint JubPoint
}

type EddsaSignature struct {
	R8 JubPoint
	S  *big.Int
}

type EncryptionKeyPairHex struct {
	PrivateKey string
	PublicKey  string
}
