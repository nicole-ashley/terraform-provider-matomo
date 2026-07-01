package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainerTag" {
			t.Errorf("method = %q, want TagManager.addContainerTag", got)
		}
		if got := r.URL.Query().Get("type"); got != "CustomHtml" {
			t.Errorf("type = %q, want CustomHtml", got)
		}
		var params map[string]string
		if err := json.Unmarshal([]byte(r.URL.Query().Get("parameters")), &params); err != nil {
			t.Fatalf("parameters not valid JSON: %v", err)
		}
		if params["customHtml"] != "<script></script>" {
			t.Errorf("parameters.customHtml = %q, want <script></script>", params["customHtml"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idtag": "5"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddContainerTag(context.Background(), 3, "abc123", "1", TagParams{
		Type:       "CustomHtml",
		Name:       "My tag",
		Parameters: map[string]string{"customHtml": "<script></script>"},
	})
	if err != nil {
		t.Fatalf("AddContainerTag() error = %v", err)
	}
	if id != "5" {
		t.Errorf("id = %q, want 5", id)
	}
}

func TestClient_UpdateContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainerTag" {
			t.Errorf("method = %q, want TagManager.updateContainerTag", got)
		}
		if got := r.URL.Query().Get("idTag"); got != "5" {
			t.Errorf("idTag = %q, want 5", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateContainerTag(context.Background(), 3, "abc123", "1", "5", TagParams{Type: "CustomHtml", Name: "Renamed"})
	if err != nil {
		t.Fatalf("UpdateContainerTag() error = %v", err)
	}
}

func TestClient_DeleteContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainerTag" {
			t.Errorf("method = %q, want TagManager.deleteContainerTag", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainerTag(context.Background(), 3, "abc123", "1", "5"); err != nil {
		t.Fatalf("DeleteContainerTag() error = %v", err)
	}
}

func TestClient_GetContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerTag" {
			t.Errorf("method = %q, want TagManager.getContainerTag", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtag": "5", "name": "My tag", "type": "CustomHtml", "status": "active",
			"parameters": map[string]any{"customHtml": "<script></script>"},
			"fireTriggerIds": []string{"1"},
			"blockTriggerIds": []string{},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	tag, err := c.GetContainerTag(context.Background(), 3, "abc123", "1", "5")
	if err != nil {
		t.Fatalf("GetContainerTag() error = %v", err)
	}
	if tag.IDTag != "5" || tag.Status != "active" || tag.Parameters["customHtml"] != "<script></script>" {
		t.Errorf("tag = %+v, want IDTag=5 Status=active Parameters.customHtml=<script></script>", tag)
	}
	if len(tag.FireTriggerIDs) != 1 || tag.FireTriggerIDs[0] != "1" {
		t.Errorf("tag.FireTriggerIDs = %v, want [1]", tag.FireTriggerIDs)
	}
}

func TestClient_PauseResumeContainerTag(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.URL.Query().Get("method")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.PauseContainerTag(context.Background(), 3, "abc123", "1", "5"); err != nil {
		t.Fatalf("PauseContainerTag() error = %v", err)
	}
	if gotMethod != "TagManager.pauseContainerTag" {
		t.Errorf("method = %q, want TagManager.pauseContainerTag", gotMethod)
	}

	if err := c.ResumeContainerTag(context.Background(), 3, "abc123", "1", "5"); err != nil {
		t.Fatalf("ResumeContainerTag() error = %v", err)
	}
	if gotMethod != "TagManager.resumeContainerTag" {
		t.Errorf("method = %q, want TagManager.resumeContainerTag", gotMethod)
	}
}

// TestClient_AddContainerTag_apiError tests error handling for AddContainerTag
func TestClient_AddContainerTag_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result":  "error",
			"message": "tag already exists",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	_, err := c.AddContainerTag(context.Background(), 3, "abc123", "1", TagParams{
		Type: "CustomHtml",
		Name: "My tag",
	})
	if err == nil {
		t.Fatalf("AddContainerTag() error = nil, want non-nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Errorf("error type = %T, want *APIError", err)
	} else if apiErr.Message != "tag already exists" {
		t.Errorf("error message = %q, want 'tag already exists'", apiErr.Message)
	}
}
