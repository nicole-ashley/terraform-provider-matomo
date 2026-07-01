package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainerVariable" {
			t.Errorf("method = %q, want TagManager.addContainerVariable", got)
		}
		if got := r.URL.Query().Get("defaultValue"); got != "n/a" {
			t.Errorf("defaultValue = %q, want n/a", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idvariable": "9"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	defaultValue := "n/a"
	id, err := c.AddContainerVariable(context.Background(), 3, "abc123", "1", VariableParams{
		Type:         "Constant",
		Name:         "My var",
		DefaultValue: &defaultValue,
	})
	if err != nil {
		t.Fatalf("AddContainerVariable() error = %v", err)
	}
	if id != "9" {
		t.Errorf("id = %q, want 9", id)
	}
}

func TestClient_UpdateContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainerVariable" {
			t.Errorf("method = %q, want TagManager.updateContainerVariable", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateContainerVariable(context.Background(), 3, "abc123", "1", "9", VariableParams{Type: "Constant", Name: "Renamed"})
	if err != nil {
		t.Fatalf("UpdateContainerVariable() error = %v", err)
	}
}

func TestClient_DeleteContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainerVariable" {
			t.Errorf("method = %q, want TagManager.deleteContainerVariable", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainerVariable(context.Background(), 3, "abc123", "1", "9"); err != nil {
		t.Fatalf("DeleteContainerVariable() error = %v", err)
	}
}

func TestClient_GetContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerVariable" {
			t.Errorf("method = %q, want TagManager.getContainerVariable", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idvariable": "9", "name": "My var", "type": "Constant",
			"parameters": map[string]any{}, "defaultValue": "n/a",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	v, err := c.GetContainerVariable(context.Background(), 3, "abc123", "1", "9")
	if err != nil {
		t.Fatalf("GetContainerVariable() error = %v", err)
	}
	if v.IDVariable != "9" || v.DefaultValue != "n/a" {
		t.Errorf("variable = %+v, want IDVariable=9 DefaultValue=n/a", v)
	}
}

func TestClient_AddContainerVariable_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result":  "error",
			"message": "Variable could not be added",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	defaultValue := "n/a"
	_, err := c.AddContainerVariable(context.Background(), 3, "abc123", "1", VariableParams{
		Type:         "Constant",
		Name:         "My var",
		DefaultValue: &defaultValue,
	})
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("AddContainerVariable() error type = %T, want *APIError", err)
	}
}
