package tokenchecker

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	spamURLRegex    = regexp.MustCompile(`(?i)(?:https?://|www\.)`)
	spamTLDRegex    = regexp.MustCompile(`(?i)\.(?:com|io|app|org|net|co|xyz|link|site|top|in|ru|cn|me|biz|info|online|club|tech|to|cloud)(?:\W|$)`)
	spamSocialRegex = regexp.MustCompile(`(?i)\b(?:t\.me|discord\.(?:gg|com)|twitter\.com|x\.com|telegram\.(?:me|org))\b`)
	spamEmailRegex  = regexp.MustCompile(`@\S+\.\S+`)
	spamFlagRegex   = regexp.MustCompile(`[\x{1F1E6}-\x{1F1FF}]{2}`)
	spamKeycapRegex = regexp.MustCompile(`[#*0-9]\x{FE0F}?\x{20E3}`)
)

// hasExtendedPictographic stands in for /\p{Extended_Pictographic}/u, which RE2 lacks.
func hasExtendedPictographic(value string) bool {
	for _, r := range value {
		if unicode.Is(extendedPictographic, r) {
			return true
		}
	}
	return false
}

func IsBeefyStakeToken(token types.ERC20Token) bool {
	return strings.Contains(token.Name, "-Boost") && strings.Contains(token.Symbol, "-Boost")
}

func IsStakeToken(token types.ERC20Token) bool {
	return IsBeefyStakeToken(token)
}

func IsAaveToken(token types.ERC20Token) bool {
	return token.AaveToken
}

func IsKinzaToken(token types.ERC20Token) bool {
	return strings.HasPrefix(token.Name, "Kinza")
}

func IsPotentiallyVolatile(token types.ERC20Token) bool {
	if token.Erc20TokenAddress == constants.ZeroAddress {
		return false
	}
	if constants.IsNonStandardStorageToken(token.Erc20TokenAddress) {
		return false
	}
	return token.IsVolatile ||
		(!constants.IsSolanaLike(token.ChainID) &&
			!constants.IsTronLike(token.ChainID) &&
			(token.BalanceStorageOffset == nil || token.AllowanceStorageOffset == nil))
}

func IsPotentiallySpamToken(token types.ERC20Token) bool {
	combined := token.Name + " " + token.Symbol
	return spamURLRegex.MatchString(combined) ||
		spamTLDRegex.MatchString(combined) ||
		spamSocialRegex.MatchString(combined) ||
		spamEmailRegex.MatchString(combined) ||
		hasExtendedPictographic(combined) ||
		spamFlagRegex.MatchString(combined) ||
		spamKeycapRegex.MatchString(combined)
}
