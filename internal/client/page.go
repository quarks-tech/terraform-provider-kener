package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// HomePageToken is the special page_path used to address the built-in home page,
// which has an empty stored path and cannot be created or deleted.
const HomePageToken = "~home"

// MonitorTags is a list of monitor tags associated with a page. Kener accepts a
// plain array of tag strings on write, but returns an array of
// {monitor_tag, position} objects on read; UnmarshalJSON normalises both shapes
// to a slice of tags (preserving order).
type MonitorTags []string

// UnmarshalJSON accepts both Kener's read shape (an array of
// {monitor_tag, position} objects) and its write shape (an array of tag
// strings), normalising either to a slice of tags in order.
func (m *MonitorTags) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*m = nil
		return nil
	}
	// Read shape: [{"monitor_tag":"...","position":N}, ...]
	var objs []struct {
		MonitorTag string `json:"monitor_tag"`
		Tag        string `json:"tag"`
	}
	if err := json.Unmarshal(b, &objs); err == nil {
		tags := make(MonitorTags, 0, len(objs))
		for _, o := range objs {
			t := o.MonitorTag
			if t == "" {
				t = o.Tag
			}
			if t != "" {
				tags = append(tags, t)
			}
		}
		*m = tags
		return nil
	}
	// Write/echo shape: ["tag1","tag2"]
	var strs []string
	if err := json.Unmarshal(b, &strs); err != nil {
		return err
	}
	*m = strs
	return nil
}

// Page mirrors a Kener status page. Optional scalar fields are pointers so that
// nil is emitted as an explicit JSON null; server-defaulted fields use
// `omitempty` so they are omitted when unset.
type Page struct {
	// ID is the internal numeric id (response-only; addressing is by PagePath).
	ID json.Number `json:"id,omitempty"`

	PagePath   string `json:"page_path,omitempty"`
	PageTitle  string `json:"page_title,omitempty"`
	PageHeader string `json:"page_header,omitempty"`

	PageSubheader *string `json:"page_subheader,omitempty"`
	PageLogo      *string `json:"page_logo,omitempty"`

	// PageSettings is a free-form JSON object (deep-merged server-side).
	PageSettings json.RawMessage `json:"page_settings,omitempty"`

	// Monitors is the ordered list of monitor tags shown on the page. It is a
	// pointer so the three states are distinguishable on write: nil omits the
	// field (leave server value untouched), a non-nil empty slice clears all
	// monitors, and a populated slice sets them.
	Monitors *MonitorTags `json:"monitors,omitempty"`
}

type pageEnvelope struct {
	Page Page `json:"page"`
}

type pagesEnvelope struct {
	Pages []Page `json:"pages"`
}

// CreatePage creates a status page.
func (c *Client) CreatePage(ctx context.Context, p *Page) (*Page, error) {
	var out pageEnvelope
	if err := c.doJSON(ctx, http.MethodPost, "/pages", nil, p, &out); err != nil {
		return nil, err
	}
	return &out.Page, nil
}

// GetPage fetches a page by its path (use HomePageToken for the home page).
func (c *Client) GetPage(ctx context.Context, path string) (*Page, error) {
	var out pageEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/pages/"+url.PathEscape(path), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Page, nil
}

// UpdatePage applies a partial update to the page identified by path. The
// identity (path) is taken from the URL, not the body.
func (c *Client) UpdatePage(ctx context.Context, path string, p *Page) (*Page, error) {
	body := *p
	body.PagePath = ""
	body.ID = ""
	var out pageEnvelope
	if err := c.doJSON(ctx, http.MethodPatch, "/pages/"+url.PathEscape(path), nil, &body, &out); err != nil {
		return nil, err
	}
	return &out.Page, nil
}

// DeletePage removes a page by path.
func (c *Client) DeletePage(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodDelete, "/pages/"+url.PathEscape(path), nil, nil, nil)
}

// ListPages returns all pages.
func (c *Client) ListPages(ctx context.Context) ([]Page, error) {
	var out pagesEnvelope
	if err := c.doJSON(ctx, http.MethodGet, "/pages", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Pages, nil
}
