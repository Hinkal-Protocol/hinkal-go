package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hinkal-Protocol/hinkal-go/internal/api"
)

func TestGet_DecodesSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Write([]byte(`{"foo":"bar"}`))
	}))
	defer srv.Close()

	var dest struct {
		Foo string `json:"foo"`
	}
	if err := api.Get(context.Background(), srv.URL, &dest); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if dest.Foo != "bar" {
		t.Fatalf("dest.Foo = %q, want %q", dest.Foo, "bar")
	}
}

func TestGet_NilDestSkipsDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	if err := api.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatalf("Get with nil dest should ignore an unparseable body: %v", err)
	}
}

func TestGet_ErrorStatusReturnsWrappedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	err := api.Get(context.Background(), srv.URL, nil)
	if err == nil {
		t.Fatal("expected an error for HTTP 404")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %q, want it to mention the status code and body", err)
	}
}

func TestPost_SendsJSONBodyAndContentType(t *testing.T) {
	var gotBody map[string]any
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	body := map[string]any{"chainId": float64(1), "erc20Addresses": []any{"0xabc"}}
	if err := api.Post(context.Background(), srv.URL, body, nil); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["chainId"] != float64(1) {
		t.Fatalf("gotBody[chainId] = %v, want 1", gotBody["chainId"])
	}
}

func TestPatch_UsesPatchMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := api.Patch(context.Background(), srv.URL, map[string]any{"a": 1}, nil); err != nil {
		t.Fatalf("Patch: %v", err)
	}
}

func TestGet_RespectsCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := api.Get(ctx, srv.URL, nil); err == nil {
		t.Fatal("expected an error for a canceled context")
	}
}
