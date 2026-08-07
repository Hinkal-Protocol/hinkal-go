package utils_test

import (
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
)

func TestGenerateHashFromSeedPhrasesMatchesTypeScript(t *testing.T) {
	words := strings.Fields("test test test test test test test test test test test junk")
	got, err := utils.GenerateHashFromSeedPhrases(words)
	if err != nil {
		t.Fatal(err)
	}
	const want = "0x25f7beda051b1c733f9d977f28bb2568ea6ff54043a798c6a5712c29621bc9da"
	if got != want {
		t.Fatalf("seed hash = %s, want %s", got, want)
	}
}
