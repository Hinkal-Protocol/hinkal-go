package pretransaction

import (
	"encoding/base64"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	solanautils "github.com/Hinkal-Protocol/hinkal-go/internal/functions/solana"
)

const hinkalInstructionUseCPISigner = 255

func ConvertOKXToHinkalInstructions(
	okxInstructions []api.OKXSwapResponseInstruction,
	cpiSignerAccount solana.PublicKey,
) ([]solanautils.HinkalInstruction, []solana.AccountMeta, error) {
	remainingAccounts := []solana.AccountMeta{}
	hinkalInstructions := []solanautils.HinkalInstruction{}

	for _, instruction := range okxInstructions {
		accountIndexes := make([]byte, len(instruction.Accounts))
		for i, account := range instruction.Accounts {
			if account.Pubkey == cpiSignerAccount.String() {
				accountIndexes[i] = hinkalInstructionUseCPISigner
				continue
			}
			found := -1
			for j := range remainingAccounts {
				if remainingAccounts[j].PublicKey.String() == account.Pubkey {
					found = j
					break
				}
			}
			if found != -1 {
				// Solana treats duplicate accounts as writable if any reference is writable.
				if remainingAccounts[found].IsWritable != account.IsWritable {
					remainingAccounts[found].IsWritable = true
				}
				accountIndexes[i] = byte(found)
				continue
			}
			pubkey, err := solana.PublicKeyFromBase58(account.Pubkey)
			if err != nil {
				return nil, nil, err
			}
			remainingAccounts = append(remainingAccounts, solana.AccountMeta{PublicKey: pubkey, IsSigner: false, IsWritable: account.IsWritable})
			accountIndexes[i] = byte(len(remainingAccounts) - 1)
		}

		data, err := base64.StdEncoding.DecodeString(instruction.Data)
		if err != nil {
			return nil, nil, err
		}

		programIndex := -1
		for j := range remainingAccounts {
			if remainingAccounts[j].PublicKey.String() == instruction.ProgramID {
				programIndex = j
				break
			}
		}
		if programIndex == -1 {
			programID, err := solana.PublicKeyFromBase58(instruction.ProgramID)
			if err != nil {
				return nil, nil, err
			}
			remainingAccounts = append(remainingAccounts, solana.AccountMeta{PublicKey: programID, IsSigner: false, IsWritable: false})
			programIndex = len(remainingAccounts) - 1
		}
		hinkalInstructions = append(hinkalInstructions, solanautils.HinkalInstruction{
			AccountIndexes: accountIndexes,
			Data:           data,
			ProgramIndex:   programIndex,
		})
	}

	return hinkalInstructions, remainingAccounts, nil
}
