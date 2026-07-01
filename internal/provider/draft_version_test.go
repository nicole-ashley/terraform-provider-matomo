package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

func TestResolveDraftVersionID_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idcontainer": "abc123", "idsite": 3, "context": "web", "name": "Main",
			"draft": map[string]any{"idcontainerversion": "1"},
		})
	}))
	defer srv.Close()

	client := matomo.NewClient(srv.URL, "test-token", srv.Client())
	id, err := resolveDraftVersionID(context.Background(), client, 3, "abc123")
	if err != nil {
		t.Fatalf("resolveDraftVersionID() error = %v", err)
	}
	if id != "1" {
		t.Errorf("id = %q, want 1", id)
	}
}

func TestResolveDraftVersionID_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idcontainer": "abc123", "idsite": 3, "context": "web", "name": "Main",
			"draft": nil,
		})
	}))
	defer srv.Close()

	client := matomo.NewClient(srv.URL, "test-token", srv.Client())
	_, err := resolveDraftVersionID(context.Background(), client, 3, "abc123")
	if err == nil {
		t.Fatal("resolveDraftVersionID() error = nil, want error (no draft version found)")
	}
}
