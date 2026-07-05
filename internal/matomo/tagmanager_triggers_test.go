package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddContainerTrigger_sendsDescription(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 9})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainerTrigger(context.Background(), 1, "abc123", "1", TriggerParams{
		Type:        "PageView",
		Name:        "My Trigger",
		Description: "a trigger description",
		Parameters:  ParamsMap{},
	})
	if err != nil {
		t.Fatalf("AddContainerTrigger() error = %v", err)
	}
	if got := gotForm.Get("description"); got != "a trigger description" {
		t.Errorf("description = %q, want %q", got, "a trigger description")
	}
}

func TestGetContainerTrigger_decodesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtrigger":   9,
			"name":        "My Trigger",
			"type":        "PageView",
			"description": "a trigger description",
			"parameters":  map[string]any{},
			"conditions":  []any{},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	trig, err := client.GetContainerTrigger(context.Background(), 1, "abc123", "1", "9")
	if err != nil {
		t.Fatalf("GetContainerTrigger() error = %v", err)
	}
	if trig.Description != "a trigger description" {
		t.Errorf("trig.Description = %q, want %q", trig.Description, "a trigger description")
	}
}
