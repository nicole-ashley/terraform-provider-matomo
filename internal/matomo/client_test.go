package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClient_call_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.getSiteFromId" {
			t.Errorf("method = %q, want SitesManager.getSiteFromId", got)
		}
		if got := r.URL.Query().Get("token_auth"); got != "test-token" {
			t.Errorf("token_auth = %q, want test-token", got)
		}
		if got := r.URL.Query().Get("format"); got != "JSON" {
			t.Errorf("format = %q, want JSON", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idsite": 3, "name": "Example"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())

	var out struct {
		IDSite int    `json:"idsite"`
		Name   string `json:"name"`
	}
	err := c.call(context.Background(), "SitesManager.getSiteFromId", url.Values{"idSite": {"3"}}, &out)
	if err != nil {
		t.Fatalf("call() error = %v", err)
	}
	if out.IDSite != 3 || out.Name != "Example" {
		t.Errorf("out = %+v, want IDSite=3 Name=Example", out)
	}
}

func TestClient_call_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Website id Not found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())

	var out map[string]any
	err := c.call(context.Background(), "SitesManager.getSiteFromId", url.Values{"idSite": {"999"}}, &out)
	if err == nil {
		t.Fatal("call() error = nil, want APIError")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T, want *APIError", err)
	}
	if apiErr.Message != "Website id Not found" {
		t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, "Website id Not found")
	}
	if apiErr.Error() != "Website id Not found" {
		t.Errorf("apiErr.Error() = %q, want %q", apiErr.Error(), "Website id Not found")
	}
}
