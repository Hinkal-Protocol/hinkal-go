package codec

import (
	"encoding/json"

	mobileerrors "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/error-handling"

	mobiletypes "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/types"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

func DecodeFeeStructure(s string) (*types.FeeStructure, error) {
	if s == "" {
		return nil, nil
	}
	var in types.FeeStructureJSON
	if err := json.Unmarshal([]byte(s), &in); err != nil {
		return nil, mobileerrors.InvalidJSON("feeStructureJSON", err)
	}
	flat, err := DecodeBig(in.FlatFee)
	if err != nil {
		return nil, err
	}
	rate, err := DecodeBig(in.VariableRate)
	if err != nil {
		return nil, err
	}
	return &types.FeeStructure{FeeToken: in.FeeToken, FlatFee: flat, VariableRate: rate}, nil
}

func DecodeCallInfos(s string) ([]types.CallInfo, error) {
	if s == "" {
		return nil, nil
	}
	var raw []struct {
		From     string `json:"from"`
		To       string `json:"to"`
		Calldata string `json:"calldata"`
		Value    string `json:"value"`
	}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, mobileerrors.InvalidJSON("callsJSON", err)
	}
	out := make([]types.CallInfo, len(raw))
	for i, r := range raw {
		value, err := DecodeBig(r.Value)
		if err != nil {
			return nil, err
		}
		out[i] = types.CallInfo{From: r.From, To: r.To, Calldata: r.Calldata, Value: value}
	}
	return out, nil
}

func DecodeProoflessFeeStructure(s string) (types.ProoflessFeeStructure, error) {
	if s == "" {
		return types.ZeroProoflessFeeStructure(), nil
	}
	var in struct {
		FeeRecipient string `json:"feeRecipient"`
		FeeToken     string `json:"feeToken"`
		FeeAmount    string `json:"feeAmount"`
	}
	if err := json.Unmarshal([]byte(s), &in); err != nil {
		return types.ProoflessFeeStructure{}, mobileerrors.InvalidJSON("proofless feeStructureJSON", err)
	}
	feeAmount, err := DecodeBig(in.FeeAmount)
	if err != nil {
		return types.ProoflessFeeStructure{}, err
	}
	out := types.ZeroProoflessFeeStructure()
	if in.FeeRecipient != "" {
		out.FeeRecipient = in.FeeRecipient
	}
	if in.FeeToken != "" {
		out.FeeToken = in.FeeToken
	}
	out.FeeAmount = feeAmount
	return out, nil
}

func EncodeFeeStructure(feeStructure types.FeeStructure) (string, error) {
	return JSONString(types.FeeStructureJSON{
		FeeToken:     feeStructure.FeeToken,
		FlatFee:      EncodeBig(feeStructure.FlatFee),
		VariableRate: EncodeBig(feeStructure.VariableRate),
	})
}

func DecodeFeeStructureArgs(
	chainID64 int64,
	feeTokenAddr, tokenAddrsJSON string,
	callsJSON string,
	variableRateWei string,
	solanaParamsJSON string,
) (mobiletypes.FeeStructureArgs, error) {
	args := mobiletypes.FeeStructureArgs{ChainID: int(chainID64)}
	if tokenAddrsJSON != "" {
		if err := json.Unmarshal([]byte(tokenAddrsJSON), &args.TokenAddrs); err != nil {
			return mobiletypes.FeeStructureArgs{}, mobileerrors.InvalidJSON("tokenAddrsJSON", err)
		}
	}

	calls, err := DecodeCallInfos(callsJSON)
	if err != nil {
		return mobiletypes.FeeStructureArgs{}, err
	}
	args.Calls = calls

	if variableRateWei != "" {
		args.VariableRate, err = DecodeBig(variableRateWei)
		if err != nil {
			return mobiletypes.FeeStructureArgs{}, err
		}
	}

	if solanaParamsJSON != "" {
		args.SolanaParams = &api.SolanaGasEstimateParams{}
		if err := json.Unmarshal([]byte(solanaParamsJSON), args.SolanaParams); err != nil {
			return mobiletypes.FeeStructureArgs{}, mobileerrors.InvalidJSON("solanaParamsJSON", err)
		}
	} else if constants.IsSolanaLike(args.ChainID) {
		mint := feeTokenAddr
		if mint == "" && len(args.TokenAddrs) > 0 {
			mint = args.TokenAddrs[0]
		}
		args.SolanaParams = &api.SolanaGasEstimateParams{MintTo: mint}
	}
	return args, nil
}
