package receivevault

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	pretransaction "github.com/Hinkal-Protocol/hinkal-go/internal/functions/pre-transaction"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const (
	receiveVaultTag  = "HINKAL_RECEIVE_V1"
	recoveryTag      = "HINKAL_RECEIVE_RECOVERY_V1"
	solanaOwnerTag   = "HINKAL_RECEIVE_SOLANA_OWNER_V1"
	recoveryScope    = "HINKAL_RECEIVE_RECOVER"
	outputScope      = "HINKAL_RECEIVE_OUTPUT"
	accountTag       = "HINKAL_RECEIVE_ACCOUNT_V1"
	accountScope     = "HINKAL_RECEIVE_VAULT_ACCOUNT"
	tronSignedPrefix = "\x19TRON Signed Message:\n32"
)

// vaultInitCodeHash(version) is immutable once the factory registers it - safe to cache forever.
var (
	receiveVaultInitCodeHashMu    sync.Mutex
	receiveVaultInitCodeHashCache = map[string]string{}
)

func parseVersion(version string) (*big.Int, error) {
	if version == "" {
		return big.NewInt(0), nil
	}
	return utils.ParseBigInt(version)
}

func TermsFromReceiveVaultRecord(record types.ReceiveVaultRecord) types.ReceiveVaultTerms {
	return types.ReceiveVaultTerms{
		Index:   record.Index,
		Network: record.Network,
		Expiry:  record.Expiry,
		Version: record.Version,
	}
}

func receiveVaultScope(terms types.ReceiveVaultTerms) string {
	return fmt.Sprintf("%d", terms.Index)
}

func ReceiveVaultNetworkOf(chainID int) types.ReceiveVaultNetwork {
	if constants.IsSolanaLike(chainID) {
		return types.ReceiveVaultNetworkSolana
	}
	if constants.IsTronLike(chainID) {
		return types.ReceiveVaultNetworkTron
	}
	return types.ReceiveVaultNetworkEvm
}

// GetReceiveVaultExpiry is not on chain: the contract will settle an address any time, this only
// bounds the relayer's watch list and when the client stops reusing an address.
func GetReceiveVaultExpiry() *big.Int {
	return big.NewInt(utils.GetCurrentTimeInSeconds() + constants.ReceiveVaultAddressTTLSeconds)
}

func IsReceiveVaultExpired(terms types.ReceiveVaultTerms, withinSeconds ...int64) bool {
	if terms.Expiry == "" {
		return false
	}
	expiry, err := utils.ParseBigInt(terms.Expiry)
	if err != nil {
		return true
	}
	var within int64
	if len(withinSeconds) > 0 {
		within = withinSeconds[0]
	}
	return expiry.Int64() < utils.GetCurrentTimeInSeconds()+within
}

func derivedKey(tag string, parts ...string) *ecdsa.PrivateKey {
	seed := gethcrypto.Keccak256([]byte(tag + ":" + strings.Join(parts, ":")))
	return gethcrypto.ToECDSAUnsafe(seed)
}

// GetReceiveVaultRecoveryWallet derives the only key that can recover from one vault: the factory
// checks its address against the params it was deployed with. The owner signer authorizes
// recover()/output signing, so it is seeded by the aux spend key, not nk. H0/H1/stealth (see
// GetReceiveVaultParams) stay nk-derived - that's the viewing/UTXO scheme, not an authorizer.
// Vaults created under the old scheme were seeded by nk: pass it as the key to reach those.
func GetReceiveVaultRecoveryWallet(auxSpendKey string, terms types.ReceiveVaultTerms) (*ecdsa.PrivateKey, error) {
	return derivedKey(recoveryTag, auxSpendKey, receiveVaultScope(terms)), nil
}

// ReceiveVaultRandomizationSeed is the deterministic seed a vault's stealth address is built from,
// so the same terms always re-derive the same address.
func ReceiveVaultRandomizationSeed(shieldedPrivateKey string, terms types.ReceiveVaultTerms) (*big.Int, error) {
	scope := receiveVaultScope(terms)
	hash := gethcrypto.Keccak256([]byte(receiveVaultTag + ":" + shieldedPrivateKey + ":" + scope))
	return new(big.Int).SetBytes(hash), nil
}

func solanaOriginalDeployer(chainID int) (string, error) {
	originalDeployer, err := constants.OriginalDeployer(chainID)
	if err != nil || originalDeployer == "" {
		return "", fmt.Errorf("receive vault: no original deployer on chain %d", chainID)
	}
	return originalDeployer, nil
}

func GetReceiveVaultSolanaOwner(auxSpendKey string, terms types.ReceiveVaultTerms) ed25519.PrivateKey {
	seed := gethcrypto.Keccak256([]byte(solanaOwnerTag + ":" + auxSpendKey + ":" + receiveVaultScope(terms)))
	return ed25519.NewKeyFromSeed(seed)
}

func GetReceiveVaultSolanaOwnerAddress(auxSpendKey string, terms types.ReceiveVaultTerms) string {
	pub := GetReceiveVaultSolanaOwner(auxSpendKey, terms).Public().(ed25519.PublicKey)
	return base58.Encode(pub)
}

var receiveVaultParamsArgs = func() abi.Arguments {
	tupleType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "recipient", Type: "tuple", Components: []abi.ArgumentMarshaling{
			{Name: "H0x", Type: "uint256"},
			{Name: "H0y", Type: "uint256"},
			{Name: "H1x", Type: "uint256"},
			{Name: "H1y", Type: "uint256"},
			{Name: "stealthAddress", Type: "uint256"},
		}},
		{Name: "owner", Type: "address"},
		{Name: "version", Type: "uint256"},
	})
	if err != nil {
		panic(fmt.Sprintf("utils: invalid receive vault params abi: %v", err))
	}
	return abi.Arguments{{Type: tupleType}}
}()

// ReceiveVaultParamsToTuple returns the params in the shape the factory ABI takes them.
func ReceiveVaultParamsToTuple(params types.ReceiveVaultParams) (types.ReceiveVaultParamsTuple, error) {
	ownerHex, err := utils.AddressToHexFormat(params.Owner)
	if err != nil {
		return types.ReceiveVaultParamsTuple{}, err
	}
	return types.ReceiveVaultParamsTuple{
		Recipient: types.ReceiveVaultRecipientTuple{
			H0x:            params.Recipient.H0x,
			H0y:            params.Recipient.H0y,
			H1x:            params.Recipient.H1x,
			H1y:            params.Recipient.H1y,
			StealthAddress: params.Recipient.StealthAddress,
		},
		Owner:   common.HexToAddress(ownerHex),
		Version: params.Version,
	}, nil
}

// EncodeReceiveVaultParams ABI-encodes the params exactly as the factory hashes them for the salt.
func EncodeReceiveVaultParams(params types.ReceiveVaultParams) ([]byte, error) {
	tuple, err := ReceiveVaultParamsToTuple(params)
	if err != nil {
		return nil, err
	}
	return receiveVaultParamsArgs.Pack(tuple)
}

func create2Prefix(chainID int) byte {
	if constants.IsTronLike(chainID) {
		return 0x41
	}
	return 0xff
}

func ComputeReceiveVaultAddress(
	params types.ReceiveVaultParams,
	factoryAddress, initCodeHash string,
	chainID int,
) (string, error) {
	encoded, err := EncodeReceiveVaultParams(params)
	if err != nil {
		return "", err
	}
	factoryHex, err := utils.AddressToHexFormat(factoryAddress)
	if err != nil {
		return "", err
	}

	packed := []byte{create2Prefix(chainID)}
	packed = append(packed, common.HexToAddress(factoryHex).Bytes()...)
	packed = append(packed, gethcrypto.Keccak256(encoded)...)
	packed = append(packed, common.FromHex(initCodeHash)...)

	return common.BytesToAddress(gethcrypto.Keccak256(packed)[12:]).Hex(), nil
}

func GetReceiveVaultFactoryAddress(chainID int) string {
	if constants.IsSolanaLike(chainID) {
		address, err := constants.HinkalAddress(chainID)
		if err != nil {
			return ""
		}
		return address
	}
	address, err := constants.ReceiveVaultFactoryAddress(chainID)
	if err != nil {
		return ""
	}
	return address
}

// GetReceiveVaultInitCodeHash reads vaultInitCodeHash(version) from the factory once and caches it
// forever: the factory only ever appends versions, so a registered hash never changes.
func GetReceiveVaultInitCodeHash(ctx context.Context, chainID int, version *big.Int) (string, error) {
	cacheKey := fmt.Sprintf("%d:%s", chainID, version.String())

	receiveVaultInitCodeHashMu.Lock()
	cached, ok := receiveVaultInitCodeHashCache[cacheKey]
	receiveVaultInitCodeHashMu.Unlock()
	if ok {
		return cached, nil
	}

	hash, err := web3.ReceiveVaultFactoryVaultInitCodeHash(ctx, chainID, version)
	if err != nil {
		return "", err
	}

	receiveVaultInitCodeHashMu.Lock()
	receiveVaultInitCodeHashCache[cacheKey] = hash
	receiveVaultInitCodeHashMu.Unlock()
	return hash, nil
}

// GetReceiveVaultCurrentVersion is the version newly generated deposit addresses are pinned to,
// read live from the factory rather than hardcoded: it advances whenever the factory registers a
// newer HinkalReceiveVault version. Never changes the meaning of an address already issued at an
// older version - see GetReceiveVaultInitCodeHash above.
func GetReceiveVaultCurrentVersion(ctx context.Context, network types.ReceiveVaultNetwork) (*big.Int, error) {
	if network == types.ReceiveVaultNetworkSolana {
		return big.NewInt(0), nil
	}
	chainID, err := getReceiveVaultCanonicalChainID(network)
	if err != nil {
		return nil, err
	}
	versionCount, err := web3.ReceiveVaultFactoryVaultVersionCount(ctx, chainID)
	if err != nil {
		return nil, err
	}
	return new(big.Int).Sub(versionCount, big.NewInt(1)), nil
}

func IsReceiveVaultSupported(chainID int) bool { return GetReceiveVaultFactoryAddress(chainID) != "" }

// GetReceiveVaultSettleFeeParams: the first settle of an EVM/Tron vault also CREATE2-deploys it;
// Solana vaults are program PDAs.
func GetReceiveVaultSettleFeeParams(chainID int, tokenAddress string, isDeployed ...bool) ReceiveVaultSettleFeeParams {
	if constants.IsSolanaLike(chainID) {
		return ReceiveVaultSettleFeeParams{
			ExternalActionID:        types.ExternalActionReceiveVaultSettle,
			SolanaTransactionParams: &api.SolanaGasEstimateParams{MintTo: tokenAddress, RootInsertCount: 1},
		}
	}
	if len(isDeployed) > 0 && isDeployed[0] {
		return ReceiveVaultSettleFeeParams{ExternalActionID: types.ExternalActionReceiveVaultSettle}
	}
	return ReceiveVaultSettleFeeParams{ExternalActionID: types.ExternalActionReceiveVaultDeployAndSettle}
}

func GetReceiveVaultSettleFeeStructure(ctx context.Context, chainID int, tokenAddress string, isDeployed ...bool) (types.FeeStructure, error) {
	params := GetReceiveVaultSettleFeeParams(chainID, tokenAddress, isDeployed...)
	return pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tokenAddress,
		[]string{tokenAddress},
		params.ExternalActionID,
		[]types.CallInfo{},
		constants.ReceiveVaultVariableRate(),
		params.SolanaTransactionParams,
	)
}

func ReceiveVaultChainsForNetwork(network types.ReceiveVaultNetwork) []int {
	chains := make([]int, 0, len(constants.HinkalSupportedChains))
	for _, chainID := range constants.HinkalSupportedChains {
		if IsReceiveVaultSupported(chainID) && ReceiveVaultNetworkOf(chainID) == network {
			chains = append(chains, chainID)
		}
	}
	return chains
}

// Any member chain resolves to the same address, so any one will do to look up the factory/init code.
func getReceiveVaultCanonicalChainID(network types.ReceiveVaultNetwork) (int, error) {
	chains := ReceiveVaultChainsForNetwork(network)
	if len(chains) == 0 {
		return 0, fmt.Errorf("receive vault: no supported chain for network %s", network)
	}
	return chains[0], nil
}

func GetReceiveVaultAddress(ctx context.Context, params types.ReceiveVaultParams, chainID int) (string, error) {
	factoryAddress := GetReceiveVaultFactoryAddress(chainID)
	if factoryAddress == "" {
		return "", fmt.Errorf("receive vault: no factory on chain %d", chainID)
	}
	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := solanaOriginalDeployer(chainID)
		if err != nil {
			return "", err
		}
		return ComputeSolanaReceiveVaultAddress(params, factoryAddress, originalDeployer)
	}
	initCodeHash, err := GetReceiveVaultInitCodeHash(ctx, chainID, params.Version)
	if err != nil {
		return "", err
	}
	vaultAddress, err := ComputeReceiveVaultAddress(params, factoryAddress, initCodeHash, chainID)
	if err != nil {
		return "", err
	}
	if constants.IsTronLike(chainID) {
		return utils.EVMHexToTronBase58Address(vaultAddress)
	}
	return vaultAddress, nil
}

func SerializeReceiveVaultParams(params types.ReceiveVaultParams) types.ReceiveVaultParamsJSON {
	return types.ReceiveVaultParamsJSON{
		Recipient: types.StealthAddressStructureJSON{
			H0x:            params.Recipient.H0x.String(),
			H0y:            params.Recipient.H0y.String(),
			H1x:            params.Recipient.H1x.String(),
			H1y:            params.Recipient.H1y.String(),
			StealthAddress: params.Recipient.StealthAddress.String(),
		},
		Owner:   params.Owner,
		Version: params.Version.String(),
	}
}

func DeserializeReceiveVaultParams(params types.ReceiveVaultParamsJSON) (types.ReceiveVaultParams, error) {
	fields := []string{
		params.Recipient.H0x,
		params.Recipient.H0y,
		params.Recipient.H1x,
		params.Recipient.H1y,
		params.Recipient.StealthAddress,
	}
	parsed := make([]*big.Int, len(fields))
	for i, field := range fields {
		value, err := utils.ParseBigInt(field)
		if err != nil {
			return types.ReceiveVaultParams{}, err
		}
		parsed[i] = value
	}

	version, err := parseVersion(params.Version)
	if err != nil {
		return types.ReceiveVaultParams{}, err
	}

	return types.ReceiveVaultParams{
		Recipient: types.StealthAddressStructure{
			H0x:            parsed[0],
			H0y:            parsed[1],
			H1x:            parsed[2],
			H1y:            parsed[3],
			StealthAddress: parsed[4],
		},
		Owner:   params.Owner,
		Version: version,
	}, nil
}

func GetReceiveVaultRecipientInfo(params types.ReceiveVaultParams, encryptionKey string) string {
	return strings.Join([]string{
		utils.ToBeHex(params.Recipient.StealthAddress),
		params.Recipient.H0x.String(),
		params.Recipient.H0y.String(),
		params.Recipient.H1x.String(),
		params.Recipient.H1y.String(),
		encryptionKey,
	}, ",")
}

func getReceiveVaultAccountWallet(shieldedPrivateKey string) *ecdsa.PrivateKey {
	return derivedKey(accountTag, shieldedPrivateKey)
}

func GetReceiveVaultAccountKey(shieldedPrivateKey string) string {
	return gethcrypto.PubkeyToAddress(getReceiveVaultAccountWallet(shieldedPrivateKey).PublicKey).Hex()
}

func getReceiveVaultAccountDigest(accountKey string, deadline *big.Int) []byte {
	packed := []byte(accountScope)
	packed = append(packed, common.HexToAddress(accountKey).Bytes()...)
	packed = append(packed, common.LeftPadBytes(deadline.Bytes(), 32)...)
	return gethcrypto.Keccak256(packed)
}

func signPersonal(key *ecdsa.PrivateKey, digest []byte) (string, error) {
	sig, err := gethcrypto.Sign(accounts.TextHash(digest), key)
	if err != nil {
		return "", err
	}
	// crypto.Sign returns V as 0 or 1; personal_sign uses 27 or 28.
	sig[64] += 27
	return "0x" + common.Bytes2Hex(sig), nil
}

func SignReceiveVaultAccount(shieldedPrivateKey string, deadline *big.Int) (types.ReceiveVaultAccountProof, error) {
	wallet := getReceiveVaultAccountWallet(shieldedPrivateKey)
	accountKey := gethcrypto.PubkeyToAddress(wallet.PublicKey).Hex()

	signature, err := signPersonal(wallet, getReceiveVaultAccountDigest(accountKey, deadline))
	if err != nil {
		return types.ReceiveVaultAccountProof{}, err
	}
	return types.ReceiveVaultAccountProof{
		AccountKey: accountKey,
		Deadline:   deadline.String(),
		Signature:  signature,
	}, nil
}

// GetReceiveVaultOutputDigest mirrors the factory's outputDigest.
func GetReceiveVaultOutputDigest(chainID int, factoryAddress, vault string, encryptedOutput []byte) ([]byte, error) {
	factoryHex, err := utils.AddressToHexFormat(factoryAddress)
	if err != nil {
		return nil, err
	}
	vaultHex, err := utils.AddressToHexFormat(vault)
	if err != nil {
		return nil, err
	}

	packed := []byte(outputScope)
	packed = append(packed, common.LeftPadBytes(big.NewInt(int64(chainID)).Bytes(), 32)...)
	packed = append(packed, common.HexToAddress(factoryHex).Bytes()...)
	packed = append(packed, common.HexToAddress(vaultHex).Bytes()...)
	packed = append(packed, gethcrypto.Keccak256(encryptedOutput)...)
	return gethcrypto.Keccak256(packed), nil
}

// SignReceiveVaultOutput signs the note once, when the address is derived: the relayer replays it
// for every settlement but can never swap the note it carries. outputDigest binds block.chainid, so
// one signature only validates on the chain it was signed for.
func SignReceiveVaultOutput(
	ctx context.Context,
	userKeys *cryptokeys.UserKeys,
	record types.ReceiveVaultRecord,
	encryptedOutput []byte,
	chainID int,
) (string, error) {
	factoryAddress := GetReceiveVaultFactoryAddress(chainID)
	if factoryAddress == "" {
		return "", fmt.Errorf("receive vault: no factory on chain %d", chainID)
	}
	_, isLegacy, err := ResolveReceiveVaultParams(ctx, userKeys, record, chainID)
	if err != nil {
		return "", err
	}
	ownerKey, err := receiveVaultOwnerKey(userKeys, isLegacy)
	if err != nil {
		return "", err
	}

	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := solanaOriginalDeployer(chainID)
		if err != nil {
			return "", err
		}
		digest, err := GetSolanaReceiveVaultOutputDigest(factoryAddress, originalDeployer, record.VaultAddress, encryptedOutput)
		if err != nil {
			return "", err
		}
		owner := GetReceiveVaultSolanaOwner(ownerKey, TermsFromReceiveVaultRecord(record))
		return SignSolanaReceiveVaultDigest(owner, digest), nil
	}

	digest, err := GetReceiveVaultOutputDigest(chainID, factoryAddress, record.VaultAddress, encryptedOutput)
	if err != nil {
		return "", err
	}
	wallet, err := GetReceiveVaultRecoveryWallet(ownerKey, TermsFromReceiveVaultRecord(record))
	if err != nil {
		return "", err
	}
	return signPersonal(wallet, digest)
}

// GetReceiveVaultRecoveryDigest mirrors the factory's recoveryDigest. Tron hashes the message a
// second time under its own prefix, and takes the signature over that raw digest.
func GetReceiveVaultRecoveryDigest(
	chainID int,
	factoryAddress, vault, token, to string,
	deadline *big.Int,
) ([]byte, error) {
	addresses := make([][]byte, 0, 4)
	for _, address := range []string{factoryAddress, vault, token, to} {
		addressHex, err := utils.AddressToHexFormat(address)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, common.HexToAddress(addressHex).Bytes())
	}

	packed := []byte(recoveryScope)
	packed = append(packed, common.LeftPadBytes(big.NewInt(int64(chainID)).Bytes(), 32)...)
	for _, address := range addresses {
		packed = append(packed, address...)
	}
	packed = append(packed, common.LeftPadBytes(deadline.Bytes(), 32)...)

	messageHash := gethcrypto.Keccak256(packed)
	if !constants.IsTronLike(chainID) {
		return messageHash, nil
	}
	return gethcrypto.Keccak256(append([]byte(tronSignedPrefix), messageHash...)), nil
}

func SignReceiveVaultRecovery(
	ctx context.Context,
	userKeys *cryptokeys.UserKeys,
	record types.ReceiveVaultRecord,
	token, to string,
	deadline *big.Int,
	chainID int,
) (string, error) {
	factoryAddress := GetReceiveVaultFactoryAddress(chainID)
	if factoryAddress == "" {
		return "", fmt.Errorf("receive vault: no factory on chain %d", chainID)
	}
	_, isLegacy, err := ResolveReceiveVaultParams(ctx, userKeys, record, chainID)
	if err != nil {
		return "", err
	}
	ownerKey, err := receiveVaultOwnerKey(userKeys, isLegacy)
	if err != nil {
		return "", err
	}

	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := solanaOriginalDeployer(chainID)
		if err != nil {
			return "", err
		}
		digest, err := GetSolanaReceiveVaultRecoveryDigest(factoryAddress, originalDeployer, record.VaultAddress, token, to, deadline)
		if err != nil {
			return "", err
		}
		owner := GetReceiveVaultSolanaOwner(ownerKey, TermsFromReceiveVaultRecord(record))
		return SignSolanaReceiveVaultDigest(owner, digest), nil
	}

	digest, err := GetReceiveVaultRecoveryDigest(chainID, factoryAddress, record.VaultAddress, token, to, deadline)
	if err != nil {
		return "", err
	}
	wallet, err := GetReceiveVaultRecoveryWallet(ownerKey, TermsFromReceiveVaultRecord(record))
	if err != nil {
		return "", err
	}

	if !constants.IsTronLike(chainID) {
		return signPersonal(wallet, digest)
	}

	sig, err := gethcrypto.Sign(digest, wallet)
	if err != nil {
		return "", err
	}
	sig[64] += 27
	return "0x" + common.Bytes2Hex(sig), nil
}

func GetReceiveVaultRecoveryAddress(auxSpendKey string, terms types.ReceiveVaultTerms) (string, error) {
	wallet, err := GetReceiveVaultRecoveryWallet(auxSpendKey, terms)
	if err != nil {
		return "", err
	}
	return gethcrypto.PubkeyToAddress(wallet.PublicKey).Hex(), nil
}

// receiveVaultOwnerKey picks the secret the vault's owner signer was seeded with: the aux spend key
// for current vaults, the nk for ones created under the old scheme.
func receiveVaultOwnerKey(userKeys *cryptokeys.UserKeys, isLegacy bool) (string, error) {
	if isLegacy {
		return userKeys.GetShieldedPrivateKey()
	}
	return userKeys.GetAuxSpendKey()
}
