package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAvailableTagTypes_flattensCategories(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name": "Custom",
				"types": []map[string]any{
					{
						"id":          "CustomHtml",
						"name":        "Custom HTML",
						"description": "Inject custom HTML",
						"category":    "Custom",
						"parameters": []map[string]any{
							{
								"name":            "customHtml",
								"type":            "string",
								"description":     "The HTML to inject",
								"condition":       "",
								"defaultValue":    "",
								"availableValues": nil,
							},
							{
								"name":            "htmlPosition",
								"type":            "string",
								"description":     "Where to inject it",
								"condition":       "",
								"defaultValue":    "top",
								"availableValues": map[string]string{"top": "Top of page", "bottom": "Bottom of page"},
							},
						},
					},
				},
			},
			{
				"name": "Analytics",
				"types": []map[string]any{
					{"id": "MatomoAnalytics", "name": "Matomo Analytics", "description": "", "category": "Analytics", "parameters": []map[string]any{}},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	templates, err := client.GetAvailableTagTypes(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetAvailableTagTypes() error = %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("len(templates) = %d, want 2 (flattened across both categories)", len(templates))
	}
	if templates[0].ID != "CustomHtml" {
		t.Errorf("templates[0].ID = %q, want CustomHtml", templates[0].ID)
	}
	if len(templates[0].Parameters) != 2 {
		t.Fatalf("len(templates[0].Parameters) = %d, want 2", len(templates[0].Parameters))
	}
	p := templates[0].Parameters[1]
	if p.Name != "htmlPosition" || p.Type != "string" {
		t.Errorf("templates[0].Parameters[1] = %+v, want Name=htmlPosition Type=string", p)
	}
	if p.AvailableValues["top"] != "Top of page" {
		t.Errorf("templates[0].Parameters[1].AvailableValues[top] = %q, want %q", p.AvailableValues["top"], "Top of page")
	}
	if templates[1].ID != "MatomoAnalytics" {
		t.Errorf("templates[1].ID = %q, want MatomoAnalytics", templates[1].ID)
	}
}
