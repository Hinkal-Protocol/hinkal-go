package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type jsonRPCMethod struct {
	Method string `json:"method"`
}

type jsonRPCResponseEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func isJSONRPCSuccess(body []byte) bool {
	var envelope jsonRPCResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return envelope.Error == nil && envelope.Result != nil
}

// rpcFallbackTransport tries the public RPC first, then POSTs the same JSON-RPC body to the guarded backend proxy on failure or for alwaysProxy methods.
type rpcFallbackTransport struct {
	next        http.RoundTripper
	alwaysProxy map[string]bool
	proxyURL    func(routePath string) string
}

// NewFallbackHTTPClient builds an http.Client using rpcFallbackTransport.
func NewFallbackHTTPClient(timeout time.Duration, alwaysProxy map[string]bool, proxyURL func(routePath string) string) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &rpcFallbackTransport{
			next:        newTransport(),
			alwaysProxy: alwaysProxy,
			proxyURL:    proxyURL,
		},
	}
}

func (t *rpcFallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	var parsed jsonRPCMethod
	_ = json.Unmarshal(bodyBytes, &parsed)

	if !t.alwaysProxy[parsed.Method] {
		if resp, err := t.next.RoundTrip(cloneRequestWithBody(req, bodyBytes)); err == nil {
			if resp.StatusCode < 400 {
				respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
				_ = resp.Body.Close()
				if readErr == nil && isJSONRPCSuccess(respBody) {
					resp.Body = io.NopCloser(bytes.NewReader(respBody))
					resp.ContentLength = int64(len(respBody))
					return resp, nil
				}
			} else {
				_ = resp.Body.Close()
			}
		}
	}

	return t.proxyRequest(req.Context(), bodyBytes)
}

func (t *rpcFallbackTransport) proxyRequest(ctx context.Context, bodyBytes []byte) (*http.Response, error) {
	routePath, err := resolveRoutePath(ctx, false)
	if err != nil {
		return nil, err
	}

	resp, err := t.postProxy(ctx, bodyBytes, routePath)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		routePath, err = resolveRoutePath(ctx, true)
		if err != nil {
			return nil, err
		}
		return t.postProxy(ctx, bodyBytes, routePath)
	}
	return resp, nil
}

func (t *rpcFallbackTransport) postProxy(ctx context.Context, bodyBytes []byte, routePath string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.proxyURL(routePath), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return Client.Do(req)
}

func cloneRequestWithBody(req *http.Request, bodyBytes []byte) *http.Request {
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	clone.ContentLength = int64(len(bodyBytes))
	return clone
}
