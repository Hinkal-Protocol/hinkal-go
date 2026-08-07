package contractabi

import (
	"bytes"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

var (
	hinkalABIMu           sync.Mutex
	hinkalABICache        = map[int]abi.ABI{}
	hinkalWrapperABIMu    sync.Mutex
	hinkalWrapperABICache = map[int]abi.ABI{}
)

func Hinkal(chainID int) (abi.ABI, error) {
	hinkalABIMu.Lock()
	defer hinkalABIMu.Unlock()

	if cached, ok := hinkalABICache[chainID]; ok {
		return cached, nil
	}
	raw, err := constants.HinkalABIJSON(chainID)
	if err != nil {
		return abi.ABI{}, err
	}
	parsed, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		return abi.ABI{}, err
	}
	hinkalABICache[chainID] = parsed
	return parsed, nil
}

func HinkalWrapper(chainID int) (abi.ABI, error) {
	hinkalWrapperABIMu.Lock()
	defer hinkalWrapperABIMu.Unlock()

	if cached, ok := hinkalWrapperABICache[chainID]; ok {
		return cached, nil
	}
	raw, err := constants.HinkalWrapperABIJSON(chainID)
	if err != nil {
		return abi.ABI{}, err
	}
	parsed, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		return abi.ABI{}, err
	}
	hinkalWrapperABICache[chainID] = parsed
	return parsed, nil
}
