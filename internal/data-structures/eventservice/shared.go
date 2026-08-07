package eventservice

import (
	"context"
	"sync/atomic"
)

type watermark struct {
	value  atomic.Uint64
	synced atomic.Bool
}

func (w *watermark) store(v uint64) { w.value.Store(v) }

func (w *watermark) load() uint64 { return w.value.Load() }

func (w *watermark) advance(v uint64) {
	for {
		cur := w.value.Load()
		if v <= cur {
			return
		}
		if w.value.CompareAndSwap(cur, v) {
			return
		}
	}
}

func (w *watermark) syncFromAtMost(v uint64) {
	if w.synced.CompareAndSwap(false, true) {
		w.value.Store(v)
		return
	}
	for {
		cur := w.value.Load()
		if v >= cur {
			return
		}
		if w.value.CompareAndSwap(cur, v) {
			return
		}
	}
}

type cancelableListener struct {
	cancel context.CancelFunc
}

func (c *cancelableListener) start(ctx context.Context) context.Context {
	ctx, c.cancel = context.WithCancel(ctx)
	return ctx
}

func (c *cancelableListener) Clear() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func safeReorgBlock(latest, reorgDepth uint64) uint64 {
	if latest+1 <= reorgDepth {
		return 0
	}
	return latest + 1 - reorgDepth
}
