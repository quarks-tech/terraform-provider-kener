package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Monitor mirrors a Kener monitor object. Optional scalar fields are pointers so
// that a nil value is emitted as an explicit JSON null (used to clear a value),
// while fields tagged `omitempty` are dropped when nil so the server applies its
// default (on create) or keeps the existing value (on PATCH).
//
// Fields with server-side defaults (status, default_status, monitor_type,
// is_hidden, include_degraded_in_downtime, confirmation_threshold) use
// `omitempty`; freely clearable fields do not.
type Monitor struct {
	// ID is the internal numeric id (response-only; addressing is by Tag). The
	// API returns it as a JSON number; json.Number decodes it losslessly.
	ID json.Number `json:"id,omitempty"`

	Tag  string `json:"tag,omitempty"`
	Name string `json:"name,omitempty"`

	// Freely nullable fields (nil -> explicit JSON null to allow clearing).
	Description         *string         `json:"description"`
	Image               *string         `json:"image"`
	Cron                *string         `json:"cron"`
	CategoryName        *string         `json:"category_name"`
	ExternalURL         *string         `json:"external_url"`
	TypeData            json.RawMessage `json:"type_data"`
	MonitorSettingsJSON json.RawMessage `json:"monitor_settings_json"`

	// Server-defaulted fields (omit when nil so the default/existing applies).
	DefaultStatus             *string `json:"default_status,omitempty"`
	Status                    *string `json:"status,omitempty"`
	MonitorType               *string `json:"monitor_type,omitempty"`
	IncludeDegradedInDowntime *string `json:"include_degraded_in_downtime,omitempty"`
	IsHidden                  *string `json:"is_hidden,omitempty"`
	ConfirmationThreshold     *int64  `json:"confirmation_threshold,omitempty"`
}

type monitorEnvelope struct {
	Monitor Monitor `json:"monitor"`
}

type monitorsEnvelope struct {
	Monitors []Monitor `json:"monitors"`
}

// MonitorListFilters are optional query filters for ListMonitors. Empty fields
// are omitted.
type MonitorListFilters struct {
	Status       string
	CategoryName string
	MonitorType  string
	IsHidden     string
}

func (f *MonitorListFilters) values() url.Values {
	if f == nil {
		return nil
	}
	v := url.Values{}
	if f.Status != "" {
		v.Set("status", f.Status)
	}
	if f.CategoryName != "" {
		v.Set("category_name", f.CategoryName)
	}
	if f.MonitorType != "" {
		v.Set("monitor_type", f.MonitorType)
	}
	if f.IsHidden != "" {
		v.Set("is_hidden", f.IsHidden)
	}
	return v
}

// CreateMonitor creates a monitor and returns the server's view of it.
func (c *Client) CreateMonitor(ctx context.Context, m *Monitor) (*Monitor, error) {
	var out monitorEnvelope
	if err := c.doJSON(ctx, http.MethodPost, "/monitors", nil, m, &out); err != nil {
		return nil, err
	}
	return &out.Monitor, nil
}

// GetMonitor fetches a monitor by its tag.
func (c *Client) GetMonitor(ctx context.Context, tag string) (*Monitor, error) {
	var out monitorEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/monitors/"+url.PathEscape(tag), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Monitor, nil
}

// UpdateMonitor applies a partial update to the monitor identified by tag. The
// tag itself is immutable and is taken from the path, not the body.
func (c *Client) UpdateMonitor(ctx context.Context, tag string, m *Monitor) (*Monitor, error) {
	// Never send an identifier in the PATCH body.
	body := *m
	body.Tag = ""
	body.ID = ""
	var out monitorEnvelope
	if err := c.doJSON(ctx, http.MethodPatch, "/monitors/"+url.PathEscape(tag), nil, &body, &out); err != nil {
		return nil, err
	}
	return &out.Monitor, nil
}

// DeleteMonitor removes a monitor (and its cascaded data) by tag.
func (c *Client) DeleteMonitor(ctx context.Context, tag string) error {
	return c.doJSON(ctx, http.MethodDelete, "/monitors/"+url.PathEscape(tag), nil, nil, nil)
}

// ListMonitors returns all monitors matching the optional filters.
func (c *Client) ListMonitors(ctx context.Context, filters *MonitorListFilters) ([]Monitor, error) {
	var out monitorsEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/monitors", filters.values(), nil, &out); err != nil {
		return nil, err
	}
	return out.Monitors, nil
}
