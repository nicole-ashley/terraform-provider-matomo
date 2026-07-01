package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainerTrigger" {
			t.Errorf("method = %q, want TagManager.addContainerTrigger", got)
		}
		var conditions []Condition
		if err := json.Unmarshal([]byte(r.URL.Query().Get("conditions")), &conditions); err != nil {
			t.Fatalf("conditions not valid JSON: %v", err)
		}
		if len(conditions) != 1 || conditions[0].Comparison != "equals" {
			t.Errorf("conditions = %+v, want one equals condition", conditions)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idtrigger": "7"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddContainerTrigger(context.Background(), 3, "abc123", "1", TriggerParams{
		Type: "PageView",
		Name: "All page views",
		Conditions: []Condition{
			{Comparison: "equals", ActualValueVariableID: "url_path", ExpectedValue: "/checkout"},
		},
	})
	if err != nil {
		t.Fatalf("AddContainerTrigger() error = %v", err)
	}
	if id != "7" {
		t.Errorf("id = %q, want 7", id)
	}
}

func TestClient_AddContainerTrigger_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Invalid trigger type"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	_, err := c.AddContainerTrigger(context.Background(), 3, "abc123", "1", TriggerParams{Type: "InvalidType"})
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("AddContainerTrigger() error type = %T, want *APIError", err)
	}
}

func TestClient_UpdateContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainerTrigger" {
			t.Errorf("method = %q, want TagManager.updateContainerTrigger", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateContainerTrigger(context.Background(), 3, "abc123", "1", "7", TriggerParams{Type: "PageView", Name: "Renamed"})
	if err != nil {
		t.Fatalf("UpdateContainerTrigger() error = %v", err)
	}
}

func TestClient_DeleteContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainerTrigger" {
			t.Errorf("method = %q, want TagManager.deleteContainerTrigger", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainerTrigger(context.Background(), 3, "abc123", "1", "7"); err != nil {
		t.Fatalf("DeleteContainerTrigger() error = %v", err)
	}
}

func TestClient_GetContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerTrigger" {
			t.Errorf("method = %q, want TagManager.getContainerTrigger", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtrigger": "7", "name": "All page views", "type": "PageView",
			"parameters": map[string]any{},
			"conditions": []map[string]any{},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	trig, err := c.GetContainerTrigger(context.Background(), 3, "abc123", "1", "7")
	if err != nil {
		t.Fatalf("GetContainerTrigger() error = %v", err)
	}
	if trig.IDTrigger != "7" || trig.Type != "PageView" {
		t.Errorf("trigger = %+v, want IDTrigger=7 Type=PageView", trig)
	}
}
