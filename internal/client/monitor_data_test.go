package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSeedMonitorData(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody MonitorDataRange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = io.WriteString(w, `{"message":"Monitoring data updated successfully","updated_count":1440}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	dev := int64(30)
	n, err := c.SeedMonitorData(context.Background(), "my-service", &MonitorDataRange{
		StartTS:   1000,
		EndTS:     2000,
		Status:    "UP",
		Latency:   150,
		Deviation: &dev,
	})
	if err != nil {
		t.Fatalf("SeedMonitorData: %v", err)
	}
	if n != 1440 {
		t.Errorf("updated_count = %d, want 1440", n)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/api/v4/monitors/my-service/data" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.StartTS != 1000 || gotBody.EndTS != 2000 || gotBody.Status != "UP" || gotBody.Latency != 150 {
		t.Errorf("unexpected body: %+v", gotBody)
	}
	if gotBody.Deviation == nil || *gotBody.Deviation != 30 {
		t.Errorf("deviation = %v, want 30", gotBody.Deviation)
	}
}

func TestSeedMonitorDataOmitsNilDeviation(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &raw)
		_, _ = io.WriteString(w, `{"updated_count":0}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	if _, err := c.SeedMonitorData(context.Background(), "svc", &MonitorDataRange{StartTS: 1, EndTS: 2, Status: "UP"}); err != nil {
		t.Fatalf("SeedMonitorData: %v", err)
	}
	if _, ok := raw["deviation"]; ok {
		t.Errorf("deviation should be omitted when nil, body = %v", raw)
	}
}

func TestSeedMonitorDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":"BAD_REQUEST","message":"start_ts must be less than end_ts"}}`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.SeedMonitorData(context.Background(), "svc", &MonitorDataRange{StartTS: 2, EndTS: 1, Status: "UP"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Code != "BAD_REQUEST" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}
