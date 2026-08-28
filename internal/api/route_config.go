package api

import (
	"context"
	"sync"

	"github.com/Hinkal-Protocol/hinkal-go/internal/constants"
)

type routeConfigResponse struct {
	A string `json:"a"`
}

var (
	routeConfigMu   sync.Mutex
	cachedRoutePath string
	routePathCached bool
)

// routeConfigURL is a var (not a call inlined below) so tests can point it at a mock server.
var routeConfigURL = func() string { return constants.GetServerURL() + constants.ServerConfig.RouteConfig }

// resolveRoutePath caches GET /rc's opaque path, refetching when forceRefresh is set.
func resolveRoutePath(ctx context.Context, forceRefresh bool) (string, error) {
	routeConfigMu.Lock()
	defer routeConfigMu.Unlock()

	if routePathCached && !forceRefresh {
		return cachedRoutePath, nil
	}

	var resp routeConfigResponse
	if err := Get(ctx, routeConfigURL(), &resp); err != nil {
		return "", err
	}
	cachedRoutePath = resp.A
	routePathCached = true
	return cachedRoutePath, nil
}
