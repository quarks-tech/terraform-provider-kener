package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMonitorTagsUnmarshal(t *testing.T) {
	cases := map[string]struct {
		in   string
		want []string
	}{
		"read shape (objects)": {
			in:   `[{"monitor_tag":"a","position":0},{"monitor_tag":"b","position":1}]`,
			want: []string{"a", "b"},
		},
		"write shape (strings)": {
			in:   `["a","b"]`,
			want: []string{"a", "b"},
		},
		"null": {
			in:   `null`,
			want: nil,
		},
		"empty": {
			in:   `[]`,
			want: []string{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var m MonitorTags
			if err := json.Unmarshal([]byte(tc.in), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(m) != len(tc.want) {
				t.Fatalf("got %v, want %v", m, tc.want)
			}
			for i := range tc.want {
				if m[i] != tc.want[i] {
					t.Errorf("index %d: got %q want %q", i, m[i], tc.want[i])
				}
			}
		})
	}
}

func TestMonitorTagsMarshal(t *testing.T) {
	// On write, monitor tags must serialise as a plain array of strings.
	empty := MonitorTags{}
	p := Page{PagePath: "x", PageTitle: "t", PageHeader: "h", Monitors: &empty}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"monitors":[]`) {
		t.Errorf("expected monitors:[] in %s", string(b))
	}

	tags := MonitorTags{"a", "b"}
	p.Monitors = &tags
	b, _ = json.Marshal(p)
	if !strings.Contains(string(b), `"monitors":["a","b"]`) {
		t.Errorf("expected monitors array in %s", string(b))
	}
}

func TestCreateAndGetPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/pages":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"page":{"id":3,"page_path":"status","page_title":"S","page_header":"H","monitors":[{"monitor_tag":"a","position":0}]}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/pages/status":
			_, _ = io.WriteString(w, `{"page":{"id":3,"page_path":"status","page_title":"S","page_header":"H","monitors":[{"monitor_tag":"a","position":0}]}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"nope"}}`)
		}
	}))
	defer srv.Close()

	c := testClient(t, srv)
	created, err := c.CreatePage(context.Background(), &Page{PagePath: "status", PageTitle: "S", PageHeader: "H"})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if created.ID.String() != "3" || created.PagePath != "status" {
		t.Errorf("unexpected created page: %+v", created)
	}
	if created.Monitors == nil || len(*created.Monitors) != 1 || (*created.Monitors)[0] != "a" {
		t.Errorf("expected monitors [a], got %+v", created.Monitors)
	}

	got, err := c.GetPage(context.Background(), "status")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if got.PageTitle != "S" {
		t.Errorf("title = %q", got.PageTitle)
	}
}
