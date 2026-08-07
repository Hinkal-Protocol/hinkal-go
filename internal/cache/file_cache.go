package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const defaultFileCacheName = "hinkalCache.json"

type FileCacheDevice struct {
	mu       sync.RWMutex
	filePath string
	store    map[string]string
}

func NewFileCacheDevice(filePath string) *FileCacheDevice {
	if filePath == "" {
		wd, err := os.Getwd()
		if err == nil {
			filePath = filepath.Join(wd, defaultFileCacheName)
		} else {
			filePath = defaultFileCacheName
		}
	}
	d := &FileCacheDevice{
		filePath: filePath,
		store:    map[string]string{},
	}
	d.readFileLocked()
	return d
}

func (d *FileCacheDevice) Get(key string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.store[key]
	return v, ok
}

func (d *FileCacheDevice) Set(key, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.readFileLocked()
	d.store[key] = value
	d.writeFileLocked()
}

func (d *FileCacheDevice) readFileLocked() {
	if d.filePath == "" {
		return
	}
	raw, err := os.ReadFile(d.filePath)
	if err != nil || len(raw) == 0 {
		return
	}
	var parsed map[string]string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return
	}
	d.store = parsed
}

func (d *FileCacheDevice) writeFileLocked() {
	if d.filePath == "" {
		return
	}
	raw, err := json.Marshal(d.store)
	if err != nil {
		return
	}
	dir := filepath.Dir(d.filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
	}
	tmpPath := d.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmpPath, d.filePath); err != nil {
		return
	}
}
