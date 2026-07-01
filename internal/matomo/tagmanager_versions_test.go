package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetContainerVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerVersions" {
			t.Errorf("method = %q, want TagManager.getContainerVersions", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			{"idcontainerversion": "2", "name": "v1", "isDraft": false},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	versions, err := c.GetContainerVersions(context.Background(), 3, "abc123")
	if err != nil {
		t.Fatalf("GetContainerVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}
	if !versions[0].IsDraft || versions[0].IDContainerVersion != "1" {
		t.Errorf("versions[0] = %+v, want draft with IDContainerVersion=1", versions[0])
	}
}

func TestClient_GetContainerVersions_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Container not found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	_, err := c.GetContainerVersions(context.Background(), 3, "abc123")
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("GetContainerVersions() error type = %T, want *APIError", err)
	}
}
