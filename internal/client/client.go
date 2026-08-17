// Package client is a standalone, typed HTTP client for the Kener status-page
// API (v4). It has no dependency on Terraform and can be used and tested in
// isolation.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// apiBasePath is the fixed prefix under which every manageable Kener endpoint
// lives. See docs/research/kener-api.md.
const apiBasePath = "/api/v4"

const defaultUserAgent = "terraform-provider-kener"

// Client talks to a Kener instance. It is safe for concurrent use.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	token      string
	userAgent  string

	// maxRetries is the number of additional attempts made on retryable
	// responses (HTTP 429 and 5xx) and transient network errors.
	maxRetries int
	retryWait  time.Duration
}

// Option customises a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying *http.Client (e.g. to inject a custom
// transport or timeout).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithTimeout sets the request timeout on the default HTTP client. Ignored if a
// custom client is supplied via WithHTTPClient.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.httpClient.Timeout = d
		}
	}
}

// WithUserAgent overrides the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithRetries configures retry behaviour on retryable responses/errors.
func WithRetries(attempts int, wait time.Duration) Option {
	return func(c *Client) {
		if attempts >= 0 {
			c.maxRetries = attempts
		}
		if wait > 0 {
			c.retryWait = wait
		}
	}
}

// New constructs a Client. endpoint is the Kener base URL (scheme + host, e.g.
// "https://status.example.com"); the /api/v4 prefix is added automatically.
// token is the Kener API key ("kener_...").
func New(endpoint, token string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("kener: endpoint must not be empty")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("kener: api token must not be empty")
	}

	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("kener: invalid endpoint %q: %w", endpoint, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("kener: endpoint %q must include scheme and host", endpoint)
	}

	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    u,
		token:      token,
		userAgent:  defaultUserAgent,
		maxRetries: 2,
		retryWait:  500 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// resolve builds an absolute URL for a path relative to the API base
// (e.g. "/monitors/foo" -> "<base>/api/v4/monitors/foo").
func (c *Client) resolve(p string, query url.Values) string {
	rel := &url.URL{Path: apiBasePath + "/" + strings.TrimLeft(p, "/")}
	abs := c.baseURL.ResolveReference(rel)
	if len(query) > 0 {
		abs.RawQuery = query.Encode()
	}
	return abs.String()
}

// doJSON executes an HTTP request with a JSON body (may be nil) and decodes a
// JSON response into out (may be nil to discard the body). It maps non-2xx
// responses to an *APIError.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("kener: encoding request body: %w", err)
		}
	}

	target := c.resolve(path, query)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryWait * time.Duration(attempt)):
			}
		}

		var reqBody io.Reader
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, reqBody)
		if err != nil {
			return fmt.Errorf("kener: building %s %s request: %w", method, path, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("kener: %s %s: %w", method, path, err)
			continue // network error: retry
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("kener: reading %s %s response: %w", method, path, readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil || len(respBody) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("kener: decoding %s %s response: %w", method, path, err)
			}
			return nil
		}

		apiErr := parseAPIError(resp.StatusCode, respBody)
		if isRetryable(resp.StatusCode) && attempt < c.maxRetries {
			lastErr = apiErr
			continue
		}
		return apiErr
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("kener: %s %s failed after retries", method, path)
	}
	return lastErr
}

func isRetryable(status int) bool {
	return status == http.StatusTooManyRequests || (status >= 500 && status <= 599)
}
