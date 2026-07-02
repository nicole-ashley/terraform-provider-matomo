package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAvailableContexts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "web", "name": "Web"},
			{"id": "amp", "name": "AMP"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	contexts, err := client.GetAvailableContexts(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableContexts() error = %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("len(contexts) = %d, want 2", len(contexts))
	}
	if contexts[0] != (Context{ID: "web", Name: "Web"}) {
		t.Errorf("contexts[0] = %+v, want {web Web}", contexts[0])
	}
	if contexts[1] != (Context{ID: "amp", Name: "AMP"}) {
		t.Errorf("contexts[1] = %+v, want {amp AMP}", contexts[1])
	}
}

func TestGetAvailableEnvironments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "live", "name": "Live"},
			{"id": "dev", "name": "Dev"},
			{"id": "staging", "name": "Staging"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	environments, err := client.GetAvailableEnvironments(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableEnvironments() error = %v", err)
	}
	if len(environments) != 3 {
		t.Fatalf("len(environments) = %d, want 3", len(environments))
	}
	if environments[0] != (Environment{ID: "live", Name: "Live"}) {
		t.Errorf("environments[0] = %+v, want {live Live}", environments[0])
	}
}

func TestGetAvailableContexts_empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	contexts, err := client.GetAvailableContexts(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableContexts() error = %v", err)
	}
	if len(contexts) != 0 {
		t.Fatalf("len(contexts) = %d, want 0", len(contexts))
	}
}
