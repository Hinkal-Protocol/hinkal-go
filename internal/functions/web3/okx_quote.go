package web3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	solana "github.com/gagliardetto/solana-go"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/functions/utils"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var (
	errOKXSolanaOnly = errors.New("OKX is only supported on Solana")
	errOKXFetch      = errors.New("OKX API Fetch Error")
)

type OKXPrice struct {
	OutSwapAmount *big.Int
	OKXData       string
}

func GetOKXPrice(
	ctx context.Context,
	chainID int,
	inSwapToken, outSwapToken types.ERC20Token,
	inSwapAmount string,
	slippagePercentage float64,
) (OKXPrice, error) {
	if !constants.IsSolanaLike(chainID) {
		return OKXPrice{}, errOKXSolanaOnly
	}

	swapperAccountSalt, err := utils.RandomBigInt(31)
	if err != nil {
		return OKXPrice{}, err
	}
	inSwapAmountWei, err := GetAmountInWei(inSwapToken, inSwapAmount)
	if err != nil {
		return OKXPrice{}, err
	}

	hinkalAddressStr, err := constants.HinkalAddress(chainID)
	if err != nil {
		return OKXPrice{}, err
	}
	hinkalProgramAddress, err := solana.PublicKeyFromBase58(hinkalAddressStr)
	if err != nil {
		return OKXPrice{}, err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return OKXPrice{}, err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return OKXPrice{}, err
	}
	swapperAccount, err := GetSwapperAccountPublicKeyFromSalt(hinkalProgramAddress, originalDeployer, swapperAccountSalt)
	if err != nil {
		return OKXPrice{}, err
	}

	quote := api.OKXQuote{
		Amount:            inSwapAmountWei.String(),
		ChainIndex:        "501", // Solana mainnet
		FromTokenAddress:  inSwapToken.Erc20TokenAddress,
		ToTokenAddress:    outSwapToken.Erc20TokenAddress,
		UserWalletAddress: swapperAccount.String(),
		SlippagePercent:   fmt.Sprintf("%v", slippagePercentage),
		DirectRoute:       true,
	}

	okxResponse, status, err := api.CallOkxAPI(ctx, quote)
	if err != nil {
		return OKXPrice{}, err
	}
	if status != "success" {
		return OKXPrice{}, errOKXFetch
	}
	if okxResponse.Code != "0" {
		return OKXPrice{}, fmt.Errorf("OKX API error: %s", okxResponse.Msg)
	}

	outSwapAmount, ok := new(big.Int).SetString(okxResponse.Data.RouterResult.ToTokenAmount, 10)
	if !ok {
		return OKXPrice{}, errOKXFetch
	}

	okxData, err := json.Marshal(struct {
		api.OKXSwapResponse
		SwapperAccountSalt string `json:"swapperAccountSalt"`
	}{OKXSwapResponse: okxResponse, SwapperAccountSalt: swapperAccountSalt.String()})
	if err != nil {
		return OKXPrice{}, err
	}

	return OKXPrice{OutSwapAmount: outSwapAmount, OKXData: string(okxData)}, nil
}
