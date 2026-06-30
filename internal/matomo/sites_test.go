package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.addSite" {
			t.Errorf("method = %q, want SitesManager.addSite", got)
		}
		if got := r.URL.Query().Get("siteName"); got != "Example" {
			t.Errorf("siteName = %q, want Example", got)
		}
		if got := r.URL.Query()["urls[]"]; len(got) != 1 || got[0] != "https://example.com" {
			t.Errorf("urls[] = %v, want [https://example.com]", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": "3"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddSite(context.Background(), AddSiteParams{
		Name: "Example",
		URLs: []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("AddSite() error = %v", err)
	}
	if id != 3 {
		t.Errorf("AddSite() id = %d, want 3", id)
	}
}

func TestClient_AddSite_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "You can't add a website"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	_, err := c.AddSite(context.Background(), AddSiteParams{Name: "Example"})
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("AddSite() error type = %T, want *APIError", err)
	}
}

func TestClient_UpdateSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.updateSite" {
			t.Errorf("method = %q, want SitesManager.updateSite", got)
		}
		if got := r.URL.Query().Get("idSite"); got != "3" {
			t.Errorf("idSite = %q, want 3", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateSite(context.Background(), 3, UpdateSiteParams{AddSiteParams{Name: "Renamed"}})
	if err != nil {
		t.Fatalf("UpdateSite() error = %v", err)
	}
}

func TestClient_DeleteSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.deleteSite" {
			t.Errorf("method = %q, want SitesManager.deleteSite", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteSite(context.Background(), 3); err != nil {
		t.Fatalf("DeleteSite() error = %v", err)
	}
}

func TestClient_GetSiteFromID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.getSiteFromId" {
			t.Errorf("method = %q, want SitesManager.getSiteFromId", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idsite": "3", "name": "Example", "timezone": "UTC", "currency": "USD",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	site, err := c.GetSiteFromID(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetSiteFromID() error = %v", err)
	}
	if site.IDSite != 3 || site.Name != "Example" {
		t.Errorf("site = %+v, want IDSite=3 Name=Example", site)
	}
}

func TestClient_GetAllSites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.getAllSites" {
			t.Errorf("method = %q, want SitesManager.getAllSites", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idsite": "1", "name": "A"},
			{"idsite": "2", "name": "B"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	sites, err := c.GetAllSites(context.Background())
	if err != nil {
		t.Fatalf("GetAllSites() error = %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("len(sites) = %d, want 2", len(sites))
	}
}
