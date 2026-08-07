package eventservice

import (
	"context"
	"log"
	"time"
)

const (
	serverPollInterval = 60 * time.Second
	clientPollInterval = 3500 * time.Millisecond
)

type PollingBlockchainEventEmitter struct {
	cancelableListener
	isServer bool
}

func NewPollingBlockchainEventEmitter(isServer bool) *PollingBlockchainEventEmitter {
	return &PollingBlockchainEventEmitter{isServer: isServer}
}

func (d *PollingBlockchainEventEmitter) StartUpdateListener(ctx context.Context, emitter *BlockchainEventEmitter) {
	interval := clientPollInterval
	if d.isServer {
		interval = serverPollInterval
	}
	ctx = d.start(ctx)
	go runPolling(ctx, interval, "polling event poll", func() {
		if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber(), false); err != nil {
			log.Printf("retrieve events poll error: %v", err)
		}
	})
}

func runPolling(ctx context.Context, interval time.Duration, label string, tick func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("%s panic recovered: %v", label, r)
					}
				}()
				tick()
			}()
		}
	}
}
