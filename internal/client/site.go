package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// SiteDataEntry is a single global site-configuration key/value. value is raw
// JSON (a string for `data_type == "string"`, or an object/array for
// `data_type == "object"`).
type SiteDataEntry struct {
	Key      string          `json:"key"`
	Value    json.RawMessage `json:"value"`
	DataType string          `json:"data_type"`
}

type siteListEnvelope struct {
	SiteData []SiteDataEntry `json:"site_data"`
}

// GetSiteConfig fetches a single site-config entry by key. The endpoint returns
// a bare SiteDataEntry (no envelope).
func (c *Client) GetSiteConfig(ctx context.Context, key string) (*SiteDataEntry, error) {
	var out SiteDataEntry
	if err := c.doJSON(ctx, http.MethodGet, "/site/"+url.PathEscape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// setSiteConfigRequest is the PATCH body for a site-config key.
type setSiteConfigRequest struct {
	Value json.RawMessage `json:"value"`
}

// SetSiteConfig updates a site-config key to the given raw JSON value and
// returns the resulting entry (re-fetched for the canonical stored form).
func (c *Client) SetSiteConfig(ctx context.Context, key string, value json.RawMessage) (*SiteDataEntry, error) {
	if err := c.doJSON(ctx, http.MethodPatch, "/site/"+url.PathEscape(key), nil, setSiteConfigRequest{Value: value}, nil); err != nil {
		return nil, err
	}
	return c.GetSiteConfig(ctx, key)
}

// ListSiteConfig returns all site-config entries.
func (c *Client) ListSiteConfig(ctx context.Context) ([]SiteDataEntry, error) {
	var out siteListEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/site", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.SiteData, nil
}
