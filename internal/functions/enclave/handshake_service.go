package enclave

import (
	"context"
	"sync"
	"time"
)

const defaultHandshakeTTL = 5 * time.Minute

type EnclaveHandshakeService struct {
	ttl   time.Duration
	fetch func(ctx context.Context) (string, error)

	mu              sync.Mutex
	cachedPublicKey string
	cachedAt        time.Time
	inFlight        *handshakeCall
}

type handshakeCall struct {
	done sync.WaitGroup
	pk   string
	err  error
}

func NewEnclaveHandshakeService(ttl time.Duration) *EnclaveHandshakeService {
	if ttl <= 0 {
		ttl = defaultHandshakeTTL
	}
	return &EnclaveHandshakeService{ttl: ttl, fetch: makeHandshakeForPublicKey}
}

func (s *EnclaveHandshakeService) GetPublicKey(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.cachedPublicKey != "" && time.Since(s.cachedAt) < s.ttl {
		pk := s.cachedPublicKey
		s.mu.Unlock()
		return pk, nil
	}
	if call := s.inFlight; call != nil {
		s.mu.Unlock()
		call.done.Wait()
		return call.pk, call.err
	}

	call := &handshakeCall{}
	call.done.Add(1)
	s.inFlight = call
	s.mu.Unlock()

	pk, err := s.fetch(ctx)

	s.mu.Lock()
	if err == nil {
		s.cachedPublicKey = pk
		s.cachedAt = time.Now()
	}
	s.inFlight = nil
	s.mu.Unlock()

	call.pk = pk
	call.err = err
	call.done.Done()
	return pk, err
}

func (s *EnclaveHandshakeService) Invalidate() {
	s.mu.Lock()
	s.cachedPublicKey = ""
	s.cachedAt = time.Time{}
	s.mu.Unlock()
}

var enclaveHandshakeService = NewEnclaveHandshakeService(defaultHandshakeTTL)
