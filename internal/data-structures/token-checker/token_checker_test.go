package tokenchecker_test

import (
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	tokenchecker "github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/token-checker"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func TestIsStakeToken(t *testing.T) {
	tests := []struct {
		name  string
		token types.ERC20Token
		want  bool
	}{
		{"beefy boost token", types.ERC20Token{Name: "Moo-Boost Vault", Symbol: "moo-Boost"}, true},
		{"name has boost but symbol doesn't", types.ERC20Token{Name: "Moo-Boost Vault", Symbol: "MOO"}, false},
		{"plain token", types.ERC20Token{Name: "USD Coin", Symbol: "USDC"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenchecker.IsStakeToken(tt.token); got != tt.want {
				t.Fatalf("IsStakeToken(%+v) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestIsAaveToken(t *testing.T) {
	if !tokenchecker.IsAaveToken(types.ERC20Token{AaveToken: true}) {
		t.Fatal("expected AaveToken:true to be recognized as an Aave token")
	}
	if tokenchecker.IsAaveToken(types.ERC20Token{AaveToken: false}) {
		t.Fatal("expected AaveToken:false to not be recognized as an Aave token")
	}
}

func TestIsKinzaToken(t *testing.T) {
	if !tokenchecker.IsKinzaToken(types.ERC20Token{Name: "Kinza USDC"}) {
		t.Fatal("expected a name starting with Kinza to match")
	}
	if tokenchecker.IsKinzaToken(types.ERC20Token{Name: "USD Kinza Coin"}) {
		t.Fatal("expected Kinza in the middle of the name to NOT match (prefix only)")
	}
}

func TestIsPotentiallyVolatile(t *testing.T) {
	offset := 5
	tests := []struct {
		name  string
		token types.ERC20Token
		want  bool
	}{
		{"native/zero address is never volatile", types.ERC20Token{Erc20TokenAddress: constants.ZeroAddress}, false},
		{"explicitly flagged volatile", types.ERC20Token{Erc20TokenAddress: "0xabc", IsVolatile: true}, true},
		{
			"EVM token missing storage offsets is treated as volatile",
			types.ERC20Token{Erc20TokenAddress: "0xabc", ChainID: constants.ChainIDs.EthMainnet},
			true,
		},
		{
			"EVM token with both storage offsets known is not volatile",
			types.ERC20Token{
				Erc20TokenAddress: "0xabc", ChainID: constants.ChainIDs.EthMainnet,
				BalanceStorageOffset: &offset, AllowanceStorageOffset: &offset,
			},
			false,
		},
		{
			"solana tokens don't need storage offsets to be considered stable",
			types.ERC20Token{Erc20TokenAddress: "mint123", ChainID: constants.ChainIDs.SolanaMainnet},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenchecker.IsPotentiallyVolatile(tt.token); got != tt.want {
				t.Fatalf("IsPotentiallyVolatile(%+v) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestIsPotentiallySpamToken(t *testing.T) {
	tests := []struct {
		name  string
		token types.ERC20Token
		want  bool
	}{
		{"legit token", types.ERC20Token{Name: "USD Coin", Symbol: "USDC"}, false},
		{"contains a URL", types.ERC20Token{Name: "Claim at https://free-airdrop.xyz", Symbol: "FREE"}, true},
		{"contains a spammy TLD", types.ERC20Token{Name: "visit free-tokens.link now", Symbol: "SCAM"}, true},
		{"contains a telegram handle", types.ERC20Token{Name: "join t.me/freemoney", Symbol: "SPAM"}, true},
		{"contains an email address", types.ERC20Token{Name: "contact scam@phish.co for prize", Symbol: "PRIZE"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenchecker.IsPotentiallySpamToken(tt.token); got != tt.want {
				t.Fatalf("IsPotentiallySpamToken(%+v) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}
