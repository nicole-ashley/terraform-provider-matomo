package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainer" {
			t.Errorf("method = %q, want TagManager.addContainer", got)
		}
		if got := r.URL.Query().Get("context"); got != "web" {
			t.Errorf("context = %q, want web", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idcontainer": "abc123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddContainer(context.Background(), 3, "web", "Main", "")
	if err != nil {
		t.Fatalf("AddContainer() error = %v", err)
	}
	if id != "abc123" {
		t.Errorf("id = %q, want abc123", id)
	}
}

func TestClient_UpdateContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainer" {
			t.Errorf("method = %q, want TagManager.updateContainer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.UpdateContainer(context.Background(), 3, "abc123", "Renamed", "desc"); err != nil {
		t.Fatalf("UpdateContainer() error = %v", err)
	}
}

func TestClient_DeleteContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainer" {
			t.Errorf("method = %q, want TagManager.deleteContainer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainer(context.Background(), 3, "abc123"); err != nil {
		t.Fatalf("DeleteContainer() error = %v", err)
	}
}

func TestClient_GetContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainer" {
			t.Errorf("method = %q, want TagManager.getContainer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idcontainer": "abc123", "idsite": "3", "context": "web", "name": "Main", "description": "",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	ct, err := c.GetContainer(context.Background(), 3, "abc123")
	if err != nil {
		t.Fatalf("GetContainer() error = %v", err)
	}
	if ct.IDContainer != "abc123" || ct.Context != "web" {
		t.Errorf("container = %+v, want IDContainer=abc123 Context=web", ct)
	}
}
