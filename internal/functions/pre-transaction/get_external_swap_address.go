package pretransaction

import (
	"errors"
	"fmt"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

var errNoExternalSwapAddress = errors.New("no external Address set during swap")

func GetExternalSwapAddress(chainID int, externalActionID types.ExternalActionID) (string, error) {
	if externalActionID != types.ExternalActionLifi {
		return "", fmt.Errorf("unsupported swap external action: %s", externalActionID)
	}

	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return "", err
	}

	externalAddress := contractData.LifiExternalActionInstanceAddress
	if externalAddress == "" {
		return "", errNoExternalSwapAddress
	}
	return externalAddress, nil
}
