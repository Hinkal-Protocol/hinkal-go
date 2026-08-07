package utils

import "sync"

var chainMutexes sync.Map

func GetChainBalanceFetchingMutex(chainID int) *sync.RWMutex {
	mutex, _ := chainMutexes.LoadOrStore(chainID, &sync.RWMutex{})
	m, ok := mutex.(*sync.RWMutex)
	if !ok {
		panic("chain mutex registry holds a non-*sync.RWMutex value")
	}
	return m
}
