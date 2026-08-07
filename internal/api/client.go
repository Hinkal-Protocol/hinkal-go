package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxResponseBytes = 32 << 20

var Client = &http.Client{
	Timeout:   60 * time.Second,
	Transport: newTransport(),
}

func newTransport() *http.Transport {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t = &http.Transport{}
	} else {
		t = t.Clone()
	}
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return t
}

func Get(ctx context.Context, url string, dest any) error {
	return do(ctx, http.MethodGet, url, nil, dest)
}

func Post(ctx context.Context, url string, body, dest any) error {
	return do(ctx, http.MethodPost, url, body, dest)
}

func Patch(ctx context.Context, url string, body, dest any) error {
	return do(ctx, http.MethodPatch, url, body, dest)
}

func do(ctx context.Context, method, url string, body, dest any) (err error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := Client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	if resp.StatusCode >= 400 {
		errBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		if readErr != nil {
			errBody = nil
		}
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, errBody)
	}
	if dest != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(dest)
	}
	return nil
}
