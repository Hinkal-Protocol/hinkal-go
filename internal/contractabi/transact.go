package contractabi

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

type abiDimensions struct {
	TokenNumber     uint16
	NullifierAmount uint16
	OutputAmount    uint16
}

type abiFeeStructure struct {
	FeeToken     common.Address
	FlatFee      *big.Int
	VariableRate *big.Int
}

type abiExternalActionData struct {
	ExternalAddress        common.Address
	ExternalActionID       *big.Int `abi:"externalActionId"`
	ExternalActionMetadata []byte
}

type abiStealthAddressStructure struct {
	H0x            *big.Int
	H0y            *big.Int
	H1x            *big.Int
	H1y            *big.Int
	StealthAddress *big.Int
}

type abiHookData struct {
	PreHookContract  common.Address
	PostHookContract common.Address
	PreHookMetadata  []byte
	PostHookMetadata []byte
}

type abiCircomData struct {
	RootHashHinkal          *big.Int
	RootHashHinkalIndex     *big.Int
	Erc20TokenAddresses     []common.Address
	AmountChanges           []*big.Int
	OnChainCreation         []bool
	SlippageValues          []*big.Int
	InputNullifiers         [][]*big.Int
	OutCommitments          [][]*big.Int
	EncryptedOutputs        [][][]byte
	OnChainEncryptedOutput  []byte
	FeeStructure            abiFeeStructure
	TimeStamp               *big.Int
	StealthAddressStructure abiStealthAddressStructure
	CalldataHash            *big.Int
	EmporiumMessage         *big.Int
	PublicSignalCount       uint16
	Relay                   common.Address
	ExternalActionData      abiExternalActionData
	HookData                abiHookData
	OriginalSender          common.Address
	ExtraData               []byte
}

// abiProofSignature is the Tron-only proofSignature argument prepended to transact.
type abiProofSignature struct {
	V uint8
	R [32]byte
	S [32]byte
}

func zeroIfNil(n *big.Int) *big.Int {
	if n == nil {
		return big.NewInt(0)
	}
	return n
}

func parseBigSlice(values []string) ([]*big.Int, error) {
	out := make([]*big.Int, len(values))
	for i, v := range values {
		n, err := utils.ParseBigInt(v)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func parseBigMatrix(values [][]string) ([][]*big.Int, error) {
	out := make([][]*big.Int, len(values))
	for i, inner := range values {
		parsed, err := parseBigSlice(inner)
		if err != nil {
			return nil, err
		}
		out[i] = parsed
	}
	return out, nil
}

func hexBytesMatrix(values [][]string) [][][]byte {
	out := make([][][]byte, len(values))
	for i, inner := range values {
		out[i] = make([][]byte, len(inner))
		for j, v := range inner {
			out[i][j] = common.FromHex(v)
		}
	}
	return out
}

func toAddresses(values []string) []common.Address {
	out := make([]common.Address, len(values))
	for i, v := range values {
		out[i] = common.HexToAddress(v)
	}
	return out
}

func circomDataToABI(c types.CircomDataType) (abiCircomData, error) {
	inputNullifiers, err := parseBigMatrix(c.InputNullifiers)
	if err != nil {
		return abiCircomData{}, err
	}
	outCommitments, err := parseBigMatrix(c.OutCommitments)
	if err != nil {
		return abiCircomData{}, err
	}
	timeStamp := big.NewInt(0)
	if c.TimeStamp != "" {
		timeStamp, err = utils.ParseBigInt(c.TimeStamp)
		if err != nil {
			return abiCircomData{}, err
		}
	}

	return abiCircomData{
		RootHashHinkal:         zeroIfNil(c.RootHashHinkal),
		RootHashHinkalIndex:    zeroIfNil(c.RootHashHinkalIndex),
		Erc20TokenAddresses:    toAddresses(c.Erc20TokenAddresses),
		AmountChanges:          c.AmountChanges,
		OnChainCreation:        c.OnChainCreation,
		SlippageValues:         c.SlippageValues,
		InputNullifiers:        inputNullifiers,
		OutCommitments:         outCommitments,
		EncryptedOutputs:       hexBytesMatrix(c.EncryptedOutputs),
		OnChainEncryptedOutput: common.FromHex(c.OnChainEncryptedOutput),
		FeeStructure: abiFeeStructure{
			FeeToken:     common.HexToAddress(c.FeeStructure.FeeToken),
			FlatFee:      c.FeeStructure.FlatFee,
			VariableRate: c.FeeStructure.VariableRate,
		},
		TimeStamp: timeStamp,
		StealthAddressStructure: abiStealthAddressStructure{
			H0x:            c.StealthAddressStructure.H0x,
			H0y:            c.StealthAddressStructure.H0y,
			H1x:            c.StealthAddressStructure.H1x,
			H1y:            c.StealthAddressStructure.H1y,
			StealthAddress: c.StealthAddressStructure.StealthAddress,
		},
		CalldataHash:      zeroIfNil(c.CalldataHash),
		EmporiumMessage:   zeroIfNil(c.EmporiumMessage),
		PublicSignalCount: uint16(c.PublicSignalCount),
		Relay:             common.HexToAddress(c.Relay),
		ExternalActionData: abiExternalActionData{
			ExternalAddress:        common.HexToAddress(c.ExternalActionData.ExternalAddress),
			ExternalActionID:       zeroIfNil(c.ExternalActionData.ExternalActionID),
			ExternalActionMetadata: common.FromHex(c.ExternalActionData.ExternalActionMetadata),
		},
		HookData: abiHookData{
			PreHookContract:  common.HexToAddress(c.HookData.PreHookContract),
			PostHookContract: common.HexToAddress(c.HookData.PostHookContract),
			PreHookMetadata:  common.FromHex(c.HookData.PreHookMetadata),
			PostHookMetadata: common.FromHex(c.HookData.PostHookMetadata),
		},
		OriginalSender: common.HexToAddress(c.OriginalSender),
		ExtraData:      common.FromHex(c.ExtraData),
	}, nil
}

func zkCallDataToABI(zk types.NewZkCallDataType) (a [2]*big.Int, b [2][2]*big.Int, c [2]*big.Int, err error) {
	parse := utils.ParseBigInt
	for i := 0; i < 2; i++ {
		if a[i], err = parse(zk.A[i]); err != nil {
			return
		}
		if c[i], err = parse(zk.C[i]); err != nil {
			return
		}
		for j := 0; j < 2; j++ {
			if b[i][j], err = parse(zk.B[i][j]); err != nil {
				return
			}
		}
	}
	return
}

func toABIDimensions(d types.DimDataType) abiDimensions {
	return abiDimensions{
		TokenNumber:     uint16(d.TokenNumber),
		NullifierAmount: uint16(d.NullifierAmount),
		OutputAmount:    uint16(d.OutputAmount),
	}
}

// PackTransact ABI-encodes a call to the EVM Hinkal contract's transact method.
func PackTransact(chainID int, zkCallData types.NewZkCallDataType, dimData types.DimDataType, circomData types.CircomDataType) ([]byte, error) {
	hinkalABI, err := Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	a, b, c, err := zkCallDataToABI(zkCallData)
	if err != nil {
		return nil, err
	}
	circom, err := circomDataToABI(circomData)
	if err != nil {
		return nil, err
	}
	return hinkalABI.Pack("transact", a, b, c, toABIDimensions(dimData), circom)
}

// PackTronTransact ABI-encodes a call to the Tron Hinkal contract's transact method, which prepends
// a proofSignature argument.
func PackTronTransact(
	chainID int,
	proofV uint8,
	proofR, proofS [32]byte,
	zkCallData types.NewZkCallDataType,
	dimData types.DimDataType,
	circomData types.CircomDataType,
) ([]byte, error) {
	hinkalABI, err := Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	a, b, c, err := zkCallDataToABI(zkCallData)
	if err != nil {
		return nil, err
	}
	circom, err := circomDataToABI(circomData)
	if err != nil {
		return nil, err
	}
	proof := abiProofSignature{V: proofV, R: proofR, S: proofS}
	return hinkalABI.Pack("transact", proof, a, b, c, toABIDimensions(dimData), circom)
}
