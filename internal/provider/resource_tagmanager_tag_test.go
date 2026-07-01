package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTagResource_basic(t *testing.T) {
	tags := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerTag":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			tags[idStr] = map[string]any{
				"idtag": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"status": "active", "parameters": map[string]any{"customHtml": "<script></script>"},
				"fireTriggerIds": []string{}, "blockTriggerIds": []string{},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idtag": idStr})
		case "TagManager.updateContainerTag":
			id := r.URL.Query().Get("idTag")
			tags[id]["name"] = r.URL.Query().Get("name")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.pauseContainerTag":
			tags[r.URL.Query().Get("idTag")]["status"] = "paused"
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.resumeContainerTag":
			tags[r.URL.Query().Get("idTag")]["status"] = "active"
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerTag":
			delete(tags, r.URL.Query().Get("idTag"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerTag":
			id := r.URL.Query().Get("idTag")
			tag, ok := tags[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Tag does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(tag)
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_tag" "test" {
  container_id = "3/abc123"
  type         = "CustomHtml"
  name         = "My tag"
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "active"),
				),
			},
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_tag" "test" {
  container_id = "3/abc123"
  type         = "CustomHtml"
  name         = "My tag"
  status       = "paused"
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "paused"),
			},
		},
	})
}
