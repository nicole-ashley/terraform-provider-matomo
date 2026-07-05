package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddContainerTag_sendsDescriptionAndPriority(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 7})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainerTag(context.Background(), 1, "abc123", "1", TagParams{
		Type:        "CustomHtml",
		Name:        "My Tag",
		Description: "a tag description",
		Priority:    42,
		Parameters:  ParamsMap{},
	})
	if err != nil {
		t.Fatalf("AddContainerTag() error = %v", err)
	}
	if got := gotForm.Get("description"); got != "a tag description" {
		t.Errorf("description = %q, want %q", got, "a tag description")
	}
	if got := gotForm.Get("priority"); got != "42" {
		t.Errorf("priority = %q, want %q", got, "42")
	}
}

func TestGetContainerTag_decodesDescriptionAndPriority(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtag":             7,
			"name":              "My Tag",
			"type":              "CustomHtml",
			"status":            "active",
			"description":       "a tag description",
			"priority":          42,
			"parameters":        map[string]any{},
			"fire_trigger_ids":  []int{},
			"block_trigger_ids": []int{},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	tag, err := client.GetContainerTag(context.Background(), 1, "abc123", "1", "7")
	if err != nil {
		t.Fatalf("GetContainerTag() error = %v", err)
	}
	if tag.Description != "a tag description" {
		t.Errorf("tag.Description = %q, want %q", tag.Description, "a tag description")
	}
	if tag.Priority != 42 {
		t.Errorf("tag.Priority = %d, want 42", tag.Priority)
	}
}
