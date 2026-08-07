package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

// hinkalCacheAccessor is the slice of Hinkal that the hinkal-cache helpers need. Depending on it
// instead of ihinkal keeps this package free of an import cycle (cryptokeys -> cache -> ihinkal -> cryptokeys).
type hinkalCacheAccessor interface {
	HinkalAddress(chainID int) string
	CacheDevice() types.ICacheDevice
}

type MemoryCacheDevice struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewMemoryCacheDevice() *MemoryCacheDevice {
	return &MemoryCacheDevice{store: map[string]string{}}
}

func NewMemoryCacheDeviceWithSerialized(serialized map[string]string) *MemoryCacheDevice {
	d := NewMemoryCacheDevice()
	for key, value := range serialized {
		d.store[key] = value
	}
	return d
}

func (d *MemoryCacheDevice) Get(key string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.store[key]
	return v, ok
}

func (d *MemoryCacheDevice) Set(key, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[key] = value
}

type NoopCacheDevice struct{}

func NewNoopCacheDevice() *NoopCacheDevice {
	return &NoopCacheDevice{}
}

func (d *NoopCacheDevice) Get(_ string) (string, bool) {
	return "", false
}

func (d *NoopCacheDevice) Set(_, _ string) {}

type HinkalCacheInterface struct {
	EncryptedOutputs []*types.EncryptedOutputWithSign `json:"encryptedOutputs"`
	LastOutput       string                           `json:"lastOutput"`
}

func emptyHinkalCache() HinkalCacheInterface {
	return HinkalCacheInterface{EncryptedOutputs: []*types.EncryptedOutputWithSign{}, LastOutput: ""}
}

func shortStrings(shieldedPublicKey, hinkalAddress string) (shortPublicKey, shortHinkalAddress string) {
	return substring(shieldedPublicKey, 25), substring(hinkalAddress, 25)
}

func substring(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func GetFilePath(chainID int, shortPublicKey, shortHinkalAddress string) string {
	return fmt.Sprintf("hinkalCache-%d-%s-%s", chainID, shortPublicKey, shortHinkalAddress)
}

func GetHinkalCache(hinkal hinkalCacheAccessor, chainID int, shieldedPublicKey string) (HinkalCacheInterface, error) {
	hinkalAddress := hinkal.HinkalAddress(chainID)
	if chainID == 0 || shieldedPublicKey == "" || hinkalAddress == "" {
		return HinkalCacheInterface{}, errors.New("GetHinkalCache: incorrect arguments")
	}
	shortPublicKey, shortHinkalAddress := shortStrings(shieldedPublicKey, hinkalAddress)
	raw, ok := hinkal.CacheDevice().Get(GetFilePath(chainID, shortPublicKey, shortHinkalAddress))
	if !ok {
		return emptyHinkalCache(), nil
	}
	var cache HinkalCacheInterface
	if json.Unmarshal([]byte(raw), &cache) != nil {
		cache = emptyHinkalCache()
	}
	return cache, nil
}

func SetHinkalCache(hinkalCache HinkalCacheInterface, hinkal hinkalCacheAccessor, chainID int, shieldedPublicKey string) error {
	hinkalAddress := hinkal.HinkalAddress(chainID)
	if chainID == 0 || shieldedPublicKey == "" || hinkalAddress == "" {
		return errors.New("SetHinkalCache: incorrect arguments")
	}
	shortPublicKey, shortHinkalAddress := shortStrings(shieldedPublicKey, hinkalAddress)
	raw, err := json.Marshal(hinkalCache)
	if err != nil {
		return err
	}
	hinkal.CacheDevice().Set(GetFilePath(chainID, shortPublicKey, shortHinkalAddress), string(raw))
	return nil
}

func ResetCache(hinkal hinkalCacheAccessor, chainID int, shieldedPublicKey string) error {
	return SetHinkalCache(emptyHinkalCache(), hinkal, chainID, shieldedPublicKey)
}
