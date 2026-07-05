package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddContainer_sendsFlags(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": "abc123"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainer(context.Background(), 1, "web", "My Container", "a description", true, false, true)
	if err != nil {
		t.Fatalf("AddContainer() error = %v", err)
	}
	if got := gotForm.Get("ignoreGtmDataLayer"); got != "1" {
		t.Errorf("ignoreGtmDataLayer = %q, want 1", got)
	}
	if got := gotForm.Get("isTagFireLimitAllowedInPreviewMode"); got != "0" {
		t.Errorf("isTagFireLimitAllowedInPreviewMode = %q, want 0", got)
	}
	if got := gotForm.Get("activelySyncGtmDataLayer"); got != "1" {
		t.Errorf("activelySyncGtmDataLayer = %q, want 1", got)
	}
}

func TestGetContainer_decodesFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idcontainer":                        "abc123",
			"idsite":                             1,
			"context":                            "web",
			"name":                               "My Container",
			"description":                        "a description",
			"ignoreGtmDataLayer":                 true,
			"isTagFireLimitAllowedInPreviewMode": false,
			"activelySyncGtmDataLayer":           true,
			"draft":                              map[string]any{"idcontainerversion": 1},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	ct, err := client.GetContainer(context.Background(), 1, "abc123")
	if err != nil {
		t.Fatalf("GetContainer() error = %v", err)
	}
	if !ct.IgnoreGtmDataLayer {
		t.Error("IgnoreGtmDataLayer = false, want true")
	}
	if ct.IsTagFireLimitAllowedInPreviewMode {
		t.Error("IsTagFireLimitAllowedInPreviewMode = true, want false")
	}
	if !ct.ActivelySyncGtmDataLayer {
		t.Error("ActivelySyncGtmDataLayer = false, want true")
	}
}
