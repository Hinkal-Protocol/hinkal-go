package cache

import "sync"

type AttachableMemoryCacheDevice[T any] struct {
	mu    sync.RWMutex
	store map[string]T
}

func NewAttachableMemoryCacheDevice[T any]() *AttachableMemoryCacheDevice[T] {
	return &AttachableMemoryCacheDevice[T]{store: map[string]T{}}
}

func (d *AttachableMemoryCacheDevice[T]) Get(key string) (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.store[key]
	return v, ok
}

func (d *AttachableMemoryCacheDevice[T]) Set(key string, value T) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[key] = value
}

func (d *AttachableMemoryCacheDevice[T]) GetOrSet(key string, compute func() (T, error)) (T, error) {
	if v, ok := d.Get(key); ok {
		return v, nil
	}
	v, err := compute()
	if err != nil {
		var zero T
		return zero, err
	}
	d.Set(key, v)
	return v, nil
}
