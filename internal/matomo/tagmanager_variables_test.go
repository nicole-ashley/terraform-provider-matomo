package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddContainerVariable_sendsDescription(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 3})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainerVariable(context.Background(), 1, "abc123", "1", VariableParams{
		Type:        "Constant",
		Name:        "My Variable",
		Description: "a variable description",
		Parameters:  ParamsMap{},
	})
	if err != nil {
		t.Fatalf("AddContainerVariable() error = %v", err)
	}
	if got := gotForm.Get("description"); got != "a variable description" {
		t.Errorf("description = %q, want %q", got, "a variable description")
	}
}

func TestGetContainerVariable_decodesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idvariable":    3,
			"name":          "My Variable",
			"type":          "Constant",
			"description":   "a variable description",
			"default_value": "",
			"parameters":    map[string]any{},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	v, err := client.GetContainerVariable(context.Background(), 1, "abc123", "1", "3")
	if err != nil {
		t.Fatalf("GetContainerVariable() error = %v", err)
	}
	if v.Description != "a variable description" {
		t.Errorf("v.Description = %q, want %q", v.Description, "a variable description")
	}
}
