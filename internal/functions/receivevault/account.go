package receivevault

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/sync/singleflight"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	cryptokeys "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/crypto-keys"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	utxoimpl "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/utxo"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/addresses"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/enclave"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/tron"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/web3"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var restoreGroup singleflight.Group

var errUnexpectedAccountType = errors.New("receive vault: unexpected restored account type")

func loadAccount(
	ctx context.Context,
	userKeys *cryptokeys.UserKeys,
	shieldedPrivateKey string,
) (enclave.ReceiveVaultAccountResponse, error) {
	deadline := big.NewInt(utils.GetCurrentTimeInSeconds() + constants.ReceiveVaultSettleRequestTTLSeconds)
	proof, err := SignReceiveVaultAccount(shieldedPrivateKey, deadline)
	if err != nil {
		return enclave.ReceiveVaultAccountResponse{}, err
	}

	account, err := enclave.FetchReceiveVaultAccount(ctx, proof)
	if err != nil {
		return enclave.ReceiveVaultAccountResponse{}, err
	}

	verified := make([]types.ReceiveVaultRecord, 0, len(account.Records))
	for _, record := range account.Records {
		rebuilt, err := BuildReceiveVaultRecord(ctx, userKeys, TermsFromReceiveVaultRecord(record))
		if err != nil {
			continue
		}
		canonicalChainID, err := getReceiveVaultCanonicalChainID(record.Network)
		if err != nil {
			continue
		}
		matches := addresses.Equal(canonicalChainID, rebuilt.VaultAddress, record.VaultAddress)
		if !matches {
			continue
		}
		verified = append(verified, record)
	}
	account.Records = verified
	return account, nil
}

func fetchAccount(
	ctx context.Context,
	userKeys *cryptokeys.UserKeys,
	accountKey, shieldedPrivateKey string,
) (enclave.ReceiveVaultAccountResponse, error) {
	account, err, _ := restoreGroup.Do(accountKey, func() (any, error) {
		return loadAccount(ctx, userKeys, shieldedPrivateKey)
	})
	if err != nil {
		return enclave.ReceiveVaultAccountResponse{}, err
	}
	loaded, ok := account.(enclave.ReceiveVaultAccountResponse)
	if !ok {
		return enclave.ReceiveVaultAccountResponse{}, errUnexpectedAccountType
	}

	return loaded, nil
}

func HinkalCreateReceiveAddress(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	forceFresh ...bool,
) (types.ReceiveVaultRecord, error) {
	if !IsReceiveVaultSupported(chainID) {
		return types.ReceiveVaultRecord{}, fmt.Errorf("receive vault: no factory on chain %d", chainID)
	}

	userKeys := hinkal.GetUserKeys()
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}
	encryptionKey, err := GetReceiveVaultEncryptionKey(userKeys)
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}

	claim := receiveVaultClaim{
		userKeys:      userKeys,
		network:       ReceiveVaultNetworkOf(chainID),
		encryptionKey: encryptionKey,
		accountKey:    GetReceiveVaultAccountKey(shieldedPrivateKey),
	}

	account, err := fetchAccount(ctx, userKeys, claim.accountKey, shieldedPrivateKey)
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}
	knownRecords := account.Records

	if len(forceFresh) == 0 || !forceFresh[0] {
		if reusable, ok := newestReusableRecord(knownRecords, claim.network); ok {
			return reusable, nil
		}
	}

	for {
		claimed, err := claimFreshAddress(ctx, claim, knownRecords)
		if err != nil {
			return types.ReceiveVaultRecord{}, err
		}
		settled, err := registerRecord(ctx, claim, claimed)
		if err != nil {
			return types.ReceiveVaultRecord{}, err
		}
		if !settled {
			return claimed, nil
		}
		knownRecords = append(knownRecords, claimed)
	}
}

func registerRecord(ctx context.Context, claim receiveVaultClaim, record types.ReceiveVaultRecord) (bool, error) {
	params, err := GetReceiveVaultParams(claim.userKeys, TermsFromReceiveVaultRecord(record))
	if err != nil {
		return false, err
	}
	encryptedOutput, err := utxoimpl.EncryptEncryptionKeyAndStealthAddress(
		claim.encryptionKey,
		params.Recipient.StealthAddress,
	)
	if err != nil {
		return false, err
	}
	shieldedPrivateKey, err := claim.userKeys.GetShieldedPrivateKey()
	if err != nil {
		return false, err
	}

	chains := ReceiveVaultChainsForNetwork(record.Network)
	outputSignatures := make([]types.ReceiveVaultOutputSignature, 0, len(chains))
	for _, chainID := range chains {
		signature, err := SignReceiveVaultOutput(shieldedPrivateKey, record, encryptedOutput, chainID)
		if err != nil {
			return false, err
		}
		outputSignatures = append(outputSignatures, types.ReceiveVaultOutputSignature{
			ChainID:   chainID,
			Signature: signature,
		})
	}

	return enclave.RegisterReceiveVault(
		ctx,
		record,
		SerializeReceiveVaultParams(params),
		claim.encryptionKey,
		claim.accountKey,
		"0x"+common.Bytes2Hex(encryptedOutput),
		outputSignatures,
	)
}

func newestReusableRecord(records []types.ReceiveVaultRecord, network types.ReceiveVaultNetwork) (types.ReceiveVaultRecord, bool) {
	reusable := make([]types.ReceiveVaultRecord, 0, len(records))
	for _, record := range records {
		if record.Network != network {
			continue
		}
		reusable = append(reusable, record)
	}
	if len(reusable) == 0 {
		return types.ReceiveVaultRecord{}, false
	}
	sort.Slice(reusable, func(left, right int) bool { return reusable[left].Index > reusable[right].Index })
	return reusable[0], true
}

func claimFreshAddress(
	ctx context.Context,
	claim receiveVaultClaim,
	knownRecords []types.ReceiveVaultRecord,
) (types.ReceiveVaultRecord, error) {
	version, err := GetReceiveVaultCurrentVersion(ctx, claim.network)
	if err != nil {
		return types.ReceiveVaultRecord{}, err
	}
	nextIndex := 0
	for _, record := range knownRecords {
		if record.Network == claim.network && record.Index >= nextIndex {
			nextIndex = record.Index + 1
		}
	}
	return BuildReceiveVaultRecord(ctx, claim.userKeys, types.ReceiveVaultTerms{
		Index:   nextIndex,
		Network: claim.network,
		Version: version.String(),
	})
}

func HinkalGetReceiveVaultAccount(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
) (types.ReceiveVaultAccount, error) {
	userKeys := hinkal.GetUserKeys()
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return types.ReceiveVaultAccount{}, err
	}
	accountKey := GetReceiveVaultAccountKey(shieldedPrivateKey)

	account, err := fetchAccount(ctx, userKeys, accountKey, shieldedPrivateKey)
	if err != nil {
		return types.ReceiveVaultAccount{}, err
	}
	records := account.Records

	entries := make([]types.ReceiveVaultRecord, len(records))
	copy(entries, records)
	sort.Slice(entries, func(left, right int) bool { return entries[left].Index > entries[right].Index })

	holdings := scanTargets(ctx, records, account.Blocks)
	blockedFunds := make([]types.ReceiveVaultBlockedFund, 0, len(holdings))
	for _, holding := range holdings {
		token, err := web3.GetErc20TokenFromAPI(ctx, holding.chainID, holding.token)
		if err != nil || token == nil {
			continue
		}
		amount, err := vaultBalance(ctx, holding.record.VaultAddress, holding.chainID, *token)
		if err != nil || amount.Sign() <= 0 {
			continue
		}
		blockedFunds = append(blockedFunds, types.ReceiveVaultBlockedFund{
			Record: holding.record,
			Token:  *token,
			Amount: amount,
			Reason: holding.reason,
		})
	}

	return types.ReceiveVaultAccount{Entries: entries, BlockedFunds: blockedFunds}, nil
}

type unshieldedHolding struct {
	record  types.ReceiveVaultRecord
	chainID int
	token   string
	reason  types.ReceiveVaultBlockReason
}

func vaultBalance(ctx context.Context, vaultAddress string, chainID int, token types.ERC20Token) (*big.Int, error) {
	if constants.IsSolanaLike(chainID) {
		return solanaVaultBalance(ctx, vaultAddress, chainID, token)
	}
	addressHex, err := utils.AddressToHexFormat(vaultAddress)
	if err != nil {
		return nil, err
	}
	tokenHex, err := utils.AddressToHexFormat(token.Erc20TokenAddress)
	if err != nil {
		return nil, err
	}
	hexToken := token
	hexToken.Erc20TokenAddress = tokenHex
	return web3.GetPublicBalanceByAddress(ctx, chainID, hexToken, addressHex)
}

func solanaClient(chainID int) (*rpc.Client, error) {
	rpcURL, err := constants.RPCURL(chainID)
	if err != nil {
		return nil, err
	}
	return api.NewSolanaRPCClientWithFallback(rpcURL), nil
}

func solanaVaultBalance(
	ctx context.Context,
	vaultAddress string,
	chainID int,
	token types.ERC20Token,
) (*big.Int, error) {
	client, err := solanaClient(chainID)
	if err != nil {
		return nil, err
	}
	vault, err := solana.PublicKeyFromBase58(vaultAddress)
	if err != nil {
		return nil, err
	}
	if token.Erc20TokenAddress == constants.SolanaNativeAddress {
		balance, err := client.GetBalance(ctx, vault, rpc.CommitmentConfirmed)
		if err != nil {
			return nil, err
		}
		return new(big.Int).SetUint64(balance.Value), nil
	}

	mint, err := solana.PublicKeyFromBase58(token.Erc20TokenAddress)
	if err != nil {
		return nil, err
	}
	tokenProgram := solana.TokenProgramID
	if token.Is2022Program {
		tokenProgram = solana.Token2022ProgramID
	}
	ata, _, err := solana.FindAssociatedTokenAddressWithProgram(vault, mint, tokenProgram)
	if err != nil {
		return nil, err
	}
	balance, err := client.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed)
	if errors.Is(err, rpc.ErrNotFound) {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, err
	}
	if balance == nil || balance.Value == nil {
		return big.NewInt(0), nil
	}
	amount, ok := new(big.Int).SetString(balance.Value.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("receive vault: invalid Solana token balance %q", balance.Value.Amount)
	}
	return amount, nil
}

func solanaHeldTokenAddresses(ctx context.Context, chainID int, vaultAddress string) ([]string, error) {
	client, err := solanaClient(chainID)
	if err != nil {
		return nil, err
	}
	vault, err := solana.PublicKeyFromBase58(vaultAddress)
	if err != nil {
		return nil, err
	}
	lamports, err := client.GetBalance(ctx, vault, rpc.CommitmentConfirmed)
	if err != nil {
		return nil, err
	}

	tokens := make([]string, 0)
	seen := map[string]bool{}
	add := func(token string) {
		if seen[token] {
			return
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	if lamports.Value > 0 {
		add(constants.SolanaNativeAddress)
	}
	for _, programID := range []solana.PublicKey{solana.TokenProgramID, solana.Token2022ProgramID} {
		accounts, err := client.GetTokenAccountsByOwner(
			ctx,
			vault,
			&rpc.GetTokenAccountsConfig{ProgramId: &programID},
			&rpc.GetTokenAccountsOpts{Encoding: solana.EncodingBase64},
		)
		if err != nil {
			return nil, err
		}
		if accounts == nil {
			continue
		}
		for _, account := range accounts.Value {
			if account == nil || account.Account.Data == nil {
				continue
			}
			data := account.Account.Data.GetBinary()
			if len(data) >= 72 && binary.LittleEndian.Uint64(data[64:72]) > 0 {
				add(solana.PublicKeyFromBytes(data[:32]).String())
			}
		}
	}
	return tokens, nil
}

// discoverHeldTokenAddresses mirrors the TS receive-vault-scan.utils.ts discoverHeldTokens: every
// token address the vault currently (or, for EVM, ever) held a balance of, per network.
func discoverHeldTokenAddresses(ctx context.Context, chainID int, vaultAddress string) ([]string, error) {
	switch {
	case constants.IsSolanaLike(chainID):
		return solanaHeldTokenAddresses(ctx, chainID, vaultAddress)
	case constants.IsTronLike(chainID):
		return tronHeldTokenAddresses(ctx, chainID, vaultAddress)
	default:
		return web3.GetEvmWalletTokenAddresses(ctx, chainID, vaultAddress)
	}
}

func tronHeldTokenAddresses(ctx context.Context, chainID int, vaultAddress string) ([]string, error) {
	client, err := tron.NewWalletClient(chainID)
	if err != nil {
		return nil, err
	}
	base58Address, err := utils.ToTronBase58IfHex(vaultAddress)
	if err != nil {
		return nil, err
	}
	return client.GetHeldTokenAddresses(ctx, base58Address)
}

func scanTargets(
	ctx context.Context,
	records []types.ReceiveVaultRecord,
	blocks []types.ReceiveVaultBlock,
) []unshieldedHolding {
	holdings := make([]unshieldedHolding, 0, len(records))
	seen := map[string]bool{}

	add := func(record types.ReceiveVaultRecord, chainID int, token string, reason types.ReceiveVaultBlockReason) {
		key := fmt.Sprintf(
			"%d:%s:%s",
			chainID,
			addresses.Normalize(chainID, record.VaultAddress),
			addresses.Normalize(chainID, token),
		)
		if seen[key] {
			return
		}
		seen[key] = true
		holdings = append(holdings, unshieldedHolding{record: record, chainID: chainID, token: token, reason: reason})
	}

	for _, record := range records {
		for _, chainID := range ReceiveVaultChainsForNetwork(record.Network) {
			heldTokens, err := discoverHeldTokenAddresses(ctx, chainID, record.VaultAddress)
			if err != nil {
				log.Printf("receive vault: token discovery failed on chain %d: %v", chainID, err)
				continue
			}
			shieldable, err := web3.GetErc20TokensForChainAPI(ctx, chainID)
			if err != nil {
				log.Printf("receive vault: token registry lookup failed: %v", err)
				continue
			}
			for _, heldToken := range heldTokens {
				supported := false
				for _, token := range shieldable {
					if addresses.Equal(chainID, token.Erc20TokenAddress, heldToken) && !token.Is2022Program {
						supported = true
						break
					}
				}
				if !supported {
					add(record, chainID, heldToken, types.ReceiveVaultBlockNotShieldable)
				}
			}
		}
	}

	for _, block := range blocks {
		if record, ok := findRecordByVault(records, block.VaultAddress, block.ChainID); ok {
			add(record, block.ChainID, block.Token, types.ReceiveVaultBlockRiskySender)
		}
	}

	return holdings
}

func findRecordByVault(records []types.ReceiveVaultRecord, vaultAddress string, chainID int) (types.ReceiveVaultRecord, bool) {
	for _, record := range records {
		if addresses.Equal(chainID, record.VaultAddress, vaultAddress) {
			return record, true
		}
	}
	return types.ReceiveVaultRecord{}, false
}
