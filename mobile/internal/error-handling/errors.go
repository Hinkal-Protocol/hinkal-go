package errorhandling

import (
	"errors"
	"fmt"
)

var (
	ErrNilHostWallet      = errors.New("mobile: nil host wallet")
	ErrNilHostSigner      = errors.New("mobile: nil host signer")
	ErrNilProviderAdapter = errors.New("mobile: nil provider adapter")
	ErrNoChainID          = errors.New("mobile: no chain id")

	ErrNotEVMChain    = errors.New("mobile: not an EVM chain id")
	ErrNotSolanaChain = errors.New("mobile: not a Solana chain id")
	ErrNotTronChain   = errors.New("mobile: not a Tron chain id")

	ErrUnknownClaimableHandle = errors.New("mobile: unknown claimable handle; call FetchClaimableUtxos first")

	ErrSignatureRequired         = errors.New("mobile: signature required")
	ErrUserKeysSignatureRequired = errors.New("mobile: userKeysSignature required")
	ErrFeeStructureRequired      = errors.New("mobile: feeStructureJSON required")

	ErrEmptyAmount     = errors.New("mobile: empty amount")
	ErrEmptyAmounts    = errors.New("mobile: amountsWeiJSON must not be empty")
	ErrEmptyRecipients = errors.New("mobile: recipientsJSON must not be empty")

	ErrTokensAmountsLengthMismatch     = errors.New("mobile: tokens and amounts must be non-empty and equal length")
	ErrRecipientsAmountsLengthMismatch = errors.New("mobile: recipients and amounts must be non-empty and equal length")

	ErrNegativeConfirmations = errors.New("mobile: negative confirmations")
	ErrInvalidPrivateAddress = errors.New("mobile: invalid private address")

	ErrSignTypedDataNotSupported   = errors.New("mobile: SignTypedData not supported")
	ErrGetTransactOptsNotSupported = errors.New("mobile: GetTransactOpts not supported")

	ErrProoflessPublicFeeOnSolana = errors.New("mobile: ProoflessDepositWithPublicFee is not supported on Solana-like chains; use ProoflessDeposit")
)

type InvalidJSONError struct {
	Field string
	Err   error
}

func (e *InvalidJSONError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("mobile: invalid %s: %v", e.Field, e.Err)
	}
	return fmt.Sprintf("mobile: invalid %s", e.Field)
}

func (e *InvalidJSONError) Unwrap() error { return e.Err }

func InvalidJSON(field string, err error) error {
	return &InvalidJSONError{Field: field, Err: err}
}

type NotBase10Error struct {
	Value string
}

func (e *NotBase10Error) Error() string {
	return fmt.Sprintf("mobile: %q is not a base-10 integer", e.Value)
}

func NotBase10(value string) error {
	return &NotBase10Error{Value: value}
}

type UnknownChainError struct {
	ChainID int64
}

func (e *UnknownChainError) Error() string {
	return fmt.Sprintf("mobile: unknown chain id %d", e.ChainID)
}

func UnknownChain(chainID int64) error {
	return &UnknownChainError{ChainID: chainID}
}

func Wrap(op string, err error) error {
	return fmt.Errorf("mobile: %s: %w", op, err)
}
