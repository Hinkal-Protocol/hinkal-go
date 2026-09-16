package pretransaction

import (
	"context"

	"github.com/Hinkal-Protocol/hinkal-go/internal/data-structures/hinkal/ihinkal"
	errorhandling "github.com/Hinkal-Protocol/hinkal-go/internal/error-handling"
)

type ResolvedRelay struct {
	Relay           string
	AutoSelectRelay bool
}

// ResolveRelay defers to henclave when it builds the proof (it fetches its own relay); otherwise
// fetches client-side. skipReservation opts out of marking the picked relay pending - only for
// callers that embed the relay in the proof without it being the one that actually submits later.
func ResolveRelay(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int, skipReservation bool) (ResolvedRelay, error) {
	if hinkal.GenerateProofRemotely() {
		return ResolvedRelay{Relay: "", AutoSelectRelay: true}, nil
	}

	relay, err := hinkal.GetRandomRelay(ctx, chainID, !skipReservation)
	if err != nil {
		return ResolvedRelay{}, err
	}
	if relay == "" {
		return ResolvedRelay{}, errorhandling.ErrRelayerNotAvailable
	}
	return ResolvedRelay{Relay: relay, AutoSelectRelay: false}, nil
}
