package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// MonitorImpact ties an incident (or maintenance) to a monitor with an impact
// level. It round-trips in the same shape on read and write.
type MonitorImpact struct {
	MonitorTag string `json:"monitor_tag"`
	// Impact is one of UP, DOWN, DEGRADED, MAINTENANCE. Omitted on write, Kener
	// defaults it to DOWN.
	Impact string `json:"impact,omitempty"`
}

// Incident mirrors a Kener incident. Writable fields: title, start_date_time,
// end_date_time, monitors. The rest are server-computed.
type Incident struct {
	// ID is the numeric identifier (response-only for addressing).
	ID json.Number `json:"id,omitempty"`

	Title         string `json:"title,omitempty"`
	StartDateTime *int64 `json:"start_date_time,omitempty"`
	EndDateTime   *int64 `json:"end_date_time"`

	// Monitors is a pointer so nil omits the field, a non-nil empty slice clears
	// all impacts, and a populated slice sets them.
	Monitors *[]MonitorImpact `json:"monitors,omitempty"`

	// Computed / read-only.
	State          string `json:"state,omitempty"`
	IncidentType   string `json:"incident_type,omitempty"`
	IncidentSource string `json:"incident_source,omitempty"`
	URL            string `json:"url,omitempty"`
}

type incidentEnvelope struct {
	Incident Incident `json:"incident"`
}

type incidentsEnvelope struct {
	Incidents []Incident `json:"incidents"`
}

// CreateIncident creates an incident.
func (c *Client) CreateIncident(ctx context.Context, in *Incident) (*Incident, error) {
	var out incidentEnvelope
	if err := c.doJSON(ctx, http.MethodPost, "/incidents", nil, in, &out); err != nil {
		return nil, err
	}
	return &out.Incident, nil
}

// GetIncident fetches an incident by its numeric id.
func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	var out incidentEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/incidents/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Incident, nil
}

// UpdateIncident applies a partial update to the incident identified by id.
func (c *Client) UpdateIncident(ctx context.Context, id string, in *Incident) (*Incident, error) {
	body := *in
	body.ID = ""
	var out incidentEnvelope
	if err := c.doJSON(ctx, http.MethodPatch, "/incidents/"+url.PathEscape(id), nil, &body, &out); err != nil {
		return nil, err
	}
	return &out.Incident, nil
}

// DeleteIncident removes an incident by id.
func (c *Client) DeleteIncident(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/incidents/"+url.PathEscape(id), nil, nil, nil)
}

// ListIncidents returns all incidents.
func (c *Client) ListIncidents(ctx context.Context) ([]Incident, error) {
	var out incidentsEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/incidents", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Incidents, nil
}

// IncidentComment is a status update attached to an incident.
type IncidentComment struct {
	ID         json.Number `json:"id,omitempty"`
	IncidentID json.Number `json:"incident_id,omitempty"`
	Comment    string      `json:"comment,omitempty"`
	// State is one of INVESTIGATING, IDENTIFIED, MONITORING, RESOLVED.
	State string `json:"state,omitempty"`
	// Timestamp is unix seconds; Kener sets it to now if omitted.
	Timestamp *int64 `json:"timestamp,omitempty"`
}

type commentEnvelope struct {
	Comment IncidentComment `json:"comment"`
}

type commentsEnvelope struct {
	Comments []IncidentComment `json:"comments"`
}

func commentPath(incidentID string) string {
	return "/incidents/" + url.PathEscape(incidentID) + "/comments"
}

// CreateIncidentComment adds a comment to an incident.
func (c *Client) CreateIncidentComment(ctx context.Context, incidentID string, cm *IncidentComment) (*IncidentComment, error) {
	var out commentEnvelope
	if err := c.doJSON(ctx, http.MethodPost, commentPath(incidentID), nil, cm, &out); err != nil {
		return nil, err
	}
	return &out.Comment, nil
}

// GetIncidentComment fetches a single comment by id.
func (c *Client) GetIncidentComment(ctx context.Context, incidentID, commentID string) (*IncidentComment, error) {
	var out commentEnvelope
	if err := c.doJSON(ctx, http.MethodGet, commentPath(incidentID)+"/"+url.PathEscape(commentID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Comment, nil
}

// UpdateIncidentComment applies a partial update to a comment.
func (c *Client) UpdateIncidentComment(ctx context.Context, incidentID, commentID string, cm *IncidentComment) (*IncidentComment, error) {
	body := *cm
	body.ID = ""
	body.IncidentID = ""
	var out commentEnvelope
	if err := c.doJSON(ctx, http.MethodPatch, commentPath(incidentID)+"/"+url.PathEscape(commentID), nil, &body, &out); err != nil {
		return nil, err
	}
	return &out.Comment, nil
}

// DeleteIncidentComment removes a comment.
func (c *Client) DeleteIncidentComment(ctx context.Context, incidentID, commentID string) error {
	return c.doJSON(ctx, http.MethodDelete, commentPath(incidentID)+"/"+url.PathEscape(commentID), nil, nil, nil)
}

// ListIncidentComments returns all comments for an incident.
func (c *Client) ListIncidentComments(ctx context.Context, incidentID string) ([]IncidentComment, error) {
	var out commentsEnvelope
	if err := c.doJSON(ctx, http.MethodGet, commentPath(incidentID), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Comments, nil
}
