package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ConfigureNewCustomDimension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "CustomDimensions.configureNewCustomDimension" {
			t.Errorf("method = %q, want CustomDimensions.configureNewCustomDimension", got)
		}
		if got := r.URL.Query().Get("scope"); got != "visit" {
			t.Errorf("scope = %q, want visit", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "1"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.ConfigureNewCustomDimension(context.Background(), 3, "Product Category", "visit", true)
	if err != nil {
		t.Fatalf("ConfigureNewCustomDimension() error = %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}

func TestClient_ConfigureExistingCustomDimension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "CustomDimensions.configureExistingCustomDimension" {
			t.Errorf("method = %q, want CustomDimensions.configureExistingCustomDimension", got)
		}
		if got := r.URL.Query().Get("idDimension"); got != "1" {
			t.Errorf("idDimension = %q, want 1", got)
		}
		if got := r.URL.Query().Get("active"); got != "0" {
			t.Errorf("active = %q, want 0", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.ConfigureExistingCustomDimension(context.Background(), 1, 3, "Product Category", false)
	if err != nil {
		t.Fatalf("ConfigureExistingCustomDimension() error = %v", err)
	}
}

func TestClient_GetConfiguredCustomDimensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "CustomDimensions.getConfiguredCustomDimensions" {
			t.Errorf("method = %q, want CustomDimensions.getConfiguredCustomDimensions", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "1", "name": "Product Category", "index": "1", "scope": "visit", "active": true, "case_sensitive": false},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	dims, err := c.GetConfiguredCustomDimensions(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetConfiguredCustomDimensions() error = %v", err)
	}
	if len(dims) != 1 || dims[0].Index != 1 || dims[0].Scope != "visit" {
		t.Errorf("dims = %+v, want one dimension with Index=1 Scope=visit", dims)
	}
}
