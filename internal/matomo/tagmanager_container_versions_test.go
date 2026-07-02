package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCreateContainerVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("method"); got != "TagManager.createContainerVersion" {
			t.Errorf("method = %q, want TagManager.createContainerVersion", got)
		}
		if got := r.Form.Get("idSite"); got != "1" {
			t.Errorf("idSite = %q, want 1", got)
		}
		if got := r.Form.Get("idContainer"); got != "abc123" {
			t.Errorf("idContainer = %q, want abc123", got)
		}
		if got := r.Form.Get("name"); got != "release-1" {
			t.Errorf("name = %q, want release-1", got)
		}
		if got := r.Form.Get("description"); got != "a description" {
			t.Errorf("description = %q, want %q", got, "a description")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 42})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	id, err := client.CreateContainerVersion(context.Background(), 1, "abc123", "release-1", "a description")
	if err != nil {
		t.Fatalf("CreateContainerVersion() error = %v", err)
	}
	if id != 42 {
		t.Errorf("CreateContainerVersion() = %d, want 42", id)
	}
}

func TestPublishContainerVersion(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{{"idcontainerversion": "42"}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	err := client.PublishContainerVersion(context.Background(), 1, "abc123", 42, "live")
	if err != nil {
		t.Fatalf("PublishContainerVersion() error = %v", err)
	}
	if got := gotForm.Get("method"); got != "TagManager.publishContainerVersion" {
		t.Errorf("method = %q, want TagManager.publishContainerVersion", got)
	}
	if got := gotForm.Get("idContainerVersion"); got != "42" {
		t.Errorf("idContainerVersion = %q, want 42", got)
	}
	if got := gotForm.Get("environment"); got != "live" {
		t.Errorf("environment = %q, want live", got)
	}
}

func TestEnablePreviewMode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	if err := client.EnablePreviewMode(context.Background(), 1, "abc123"); err != nil {
		t.Fatalf("EnablePreviewMode() error = %v", err)
	}
	if got := gotForm.Get("method"); got != "TagManager.enablePreviewMode" {
		t.Errorf("method = %q, want TagManager.enablePreviewMode", got)
	}
	if gotForm.Has("idContainerVersion") {
		t.Errorf("idContainerVersion should not be set, got %q", gotForm.Get("idContainerVersion"))
	}
}

func TestDisablePreviewMode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	if err := client.DisablePreviewMode(context.Background(), 1, "abc123"); err != nil {
		t.Fatalf("DisablePreviewMode() error = %v", err)
	}
	if got := gotForm.Get("method"); got != "TagManager.disablePreviewMode" {
		t.Errorf("method = %q, want TagManager.disablePreviewMode", got)
	}
}

func TestGetContainerVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("method"); got != "TagManager.getContainerVersions" {
			t.Errorf("method = %q, want TagManager.getContainerVersions", got)
		}
		if got := r.Form.Get("idSite"); got != "1" {
			t.Errorf("idSite = %q, want 1", got)
		}
		if got := r.Form.Get("idContainer"); got != "abc123" {
			t.Errorf("idContainer = %q, want abc123", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"idcontainerversion": 1, "name": "Draft", "description": ""},
			{"idcontainerversion": 42, "name": "acceptance-test-version", "description": "a description"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	versions, err := client.GetContainerVersions(context.Background(), 1, "abc123")
	if err != nil {
		t.Fatalf("GetContainerVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}
	if versions[1] != (ContainerVersion{IDContainerVersion: 42, Name: "acceptance-test-version", Description: "a description"}) {
		t.Errorf("versions[1] = %+v, want {42 acceptance-test-version a description}", versions[1])
	}
}
