package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, srv *httptest.Server, opts ...Option) *Client {
	t.Helper()
	all := append([]Option{WithRetries(2, time.Millisecond)}, opts...)
	c, err := New(srv.URL, "kener_testtoken", all...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidation(t *testing.T) {
	if _, err := New("", "tok"); err == nil {
		t.Fatal("expected error for empty endpoint")
	}
	if _, err := New("https://x", ""); err == nil {
		t.Fatal("expected error for empty token")
	}
	if _, err := New("://bad", "tok"); err == nil {
		t.Fatal("expected error for endpoint without scheme/host")
	}
	if _, err := New("https://status.example.com", "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestHeadersAndPath(t *testing.T) {
	var gotAuth, gotAccept, gotUA, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"monitor":{"id":"1","tag":"t","name":"n"}}`)
	}))
	defer srv.Close()

	c := testClient(t, srv, WithUserAgent("ua/1"))
	if _, err := c.GetMonitor(context.Background(), "t"); err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if gotAuth != "Bearer kener_testtoken" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q", gotAccept)
	}
	if gotUA != "ua/1" {
		t.Errorf("User-Agent = %q", gotUA)
	}
	if gotPath != "/api/v4/monitors/t" {
		t.Errorf("path = %q", gotPath)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":"BAD_REQUEST","message":"tag exists"}}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.CreateMonitor(context.Background(), &Monitor{Tag: "t", Name: "n"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Code != "BAD_REQUEST" || apiErr.Message != "tag exists" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}

func TestIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"monitor not found"}}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.GetMonitor(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

func TestRetryOn5xxThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"code":"INTERNAL_ERROR","message":"boom"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"monitor":{"id":"1","tag":"t","name":"n"}}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	m, err := c.GetMonitor(context.Background(), "t")
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if m.Name != "n" {
		t.Errorf("name = %q", m.Name)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", got)
	}
}

func TestRetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":{"code":"INTERNAL_ERROR","message":"down"}}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.GetMonitor(context.Background(), "t")
	if err == nil {
		t.Fatal("expected error after retries")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("expected 503 in error, got %v", err)
	}
}
