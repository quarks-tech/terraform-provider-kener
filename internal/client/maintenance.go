package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Maintenance mirrors a Kener scheduled-maintenance window. Writable on create:
// title, description, start_date_time, rrule, duration_seconds, monitors.
// `status` is settable only via update (it is ignored on create).
type Maintenance struct {
	ID json.Number `json:"id,omitempty"`

	Title           string  `json:"title,omitempty"`
	Description     *string `json:"description"`
	StartDateTime   *int64  `json:"start_date_time,omitempty"`
	RRule           string  `json:"rrule,omitempty"`
	DurationSeconds *int64  `json:"duration_seconds,omitempty"`
	Status          string  `json:"status,omitempty"`

	// Monitors is a pointer so nil omits the field, a non-nil empty slice clears
	// all impacts, and a populated slice sets them.
	Monitors *[]MonitorImpact `json:"monitors,omitempty"`

	// URL is a computed absolute public URL (carries ?type=maintenance).
	URL string `json:"url,omitempty"`
}

type maintenanceEnvelope struct {
	Maintenance Maintenance `json:"maintenance"`
}

type maintenancesEnvelope struct {
	Maintenances []Maintenance `json:"maintenances"`
}

// CreateMaintenance creates a maintenance window. The status field is never sent
// on create (Kener ignores it there); set it with UpdateMaintenance.
func (c *Client) CreateMaintenance(ctx context.Context, m *Maintenance) (*Maintenance, error) {
	body := *m
	body.ID = ""
	body.Status = ""
	var out maintenanceEnvelope
	if err := c.doJSON(ctx, http.MethodPost, "/maintenances", nil, &body, &out); err != nil {
		return nil, err
	}
	return &out.Maintenance, nil
}

// GetMaintenance fetches a maintenance by its numeric id.
func (c *Client) GetMaintenance(ctx context.Context, id string) (*Maintenance, error) {
	var out maintenanceEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/maintenances/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Maintenance, nil
}

// UpdateMaintenance applies a partial update (including status) to a maintenance.
func (c *Client) UpdateMaintenance(ctx context.Context, id string, m *Maintenance) (*Maintenance, error) {
	body := *m
	body.ID = ""
	var out maintenanceEnvelope
	if err := c.doJSON(ctx, http.MethodPatch, "/maintenances/"+url.PathEscape(id), nil, &body, &out); err != nil {
		return nil, err
	}
	return &out.Maintenance, nil
}

// DeleteMaintenance removes a maintenance by id.
func (c *Client) DeleteMaintenance(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/maintenances/"+url.PathEscape(id), nil, nil, nil)
}

// ListMaintenances returns all maintenance windows.
func (c *Client) ListMaintenances(ctx context.Context) ([]Maintenance, error) {
	var out maintenancesEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/maintenances", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Maintenances, nil
}
