package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// APIError represents a non-2xx response from the Kener API. The API returns a
// consistent envelope: {"error":{"code":"...","message":"..."}}.
type APIError struct {
	// StatusCode is the HTTP status code of the response.
	StatusCode int
	// Code is the machine-readable error code (e.g. "NOT_FOUND",
	// "BAD_REQUEST", "UNAUTHORIZED", "INTERNAL_ERROR"). May be empty if the
	// body did not follow the envelope.
	Code string
	// Message is the human-readable error message.
	Message string
	// Body is the raw response body, retained for diagnostics when the
	// envelope could not be parsed.
	Body string
}

func (e *APIError) Error() string {
	if e.Code != "" || e.Message != "" {
		return fmt.Sprintf("kener api error (http %d, code %q): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("kener api error (http %d): %s", e.StatusCode, e.Body)
}

// errorEnvelope matches Kener's error response shape.
type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseAPIError(status int, body []byte) *APIError {
	e := &APIError{StatusCode: status, Body: string(body)}
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil {
		e.Code = env.Error.Code
		e.Message = env.Error.Message
	}
	return e
}

// IsNotFound reports whether err is a 404 / NOT_FOUND API error. Resources use
// this in Read to detect out-of-band deletion and drop themselves from state.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound || apiErr.Code == "NOT_FOUND"
	}
	return false
}

// IsUnauthorized reports whether err is a 401 / UNAUTHORIZED API error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized || apiErr.Code == "UNAUTHORIZED"
	}
	return false
}
