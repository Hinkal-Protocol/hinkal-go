package eventservice

import (
	"context"
	"log"
	"time"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/blockchainevent"
	"github.com/Hinkal-Protocol/hinkal-go/internal/types"
)

const clientEventPollInterval = 5 * time.Second

type ClientBlockchainEventEmitter struct {
	cancelableListener
	eventCategory types.EventCategory
}

func NewClientBlockchainEventEmitter(eventCategory types.EventCategory) *ClientBlockchainEventEmitter {
	return &ClientBlockchainEventEmitter{eventCategory: eventCategory}
}

func (d *ClientBlockchainEventEmitter) StartUpdateListener(ctx context.Context, emitter *BlockchainEventEmitter) {
	ctx = d.start(ctx)
	fetchFrom := emitter.LatestBlockNumber() + 1
	go runPolling(ctx, clientEventPollInterval, "client event poll", func() {
		resp, err := api.GetSnapshotServerEvents(ctx, emitter.ChainID(), d.eventCategory, fetchFrom)
		if err != nil {
			log.Printf("snapshot events poll error: %v", err)
			return
		}
		if len(resp.Events) > 0 {
			events := make([]*blockchainevent.BlockchainEvent, 0, len(resp.Events))
			for _, serialized := range resp.Events {
				ev, err := blockchainevent.NewFromSerialized(serialized)
				if err != nil {
					log.Printf("deserialize event error: %v", err)
					continue
				}
				events = append(events, ev)
			}
			if err := emitter.ProcessExternalEvents(events, resp.LatestBlockNumber); err != nil {
				log.Printf("process external events error: %v", err)
				return
			}
		}
		if resp.LatestBlockNumber >= fetchFrom {
			fetchFrom = resp.LatestBlockNumber + 1
			emitter.AdvanceLatestBlockNumber(fetchFrom)
		}
	})
}
