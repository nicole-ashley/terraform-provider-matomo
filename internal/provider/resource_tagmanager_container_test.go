package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerContainerResource_basic(t *testing.T) {
	containers := map[string]map[string]any{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.addContainer":
			id := "abc123"
			containers[id] = map[string]any{
				"idcontainer": id, "idsite": r.URL.Query().Get("idSite"),
				"context": r.URL.Query().Get("context"), "name": r.URL.Query().Get("name"),
				"description": r.URL.Query().Get("description"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idcontainer": id})
		case "TagManager.updateContainer":
			id := r.URL.Query().Get("idContainer")
			containers[id]["name"] = r.URL.Query().Get("name")
			containers[id]["description"] = r.URL.Query().Get("description")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainer":
			delete(containers, r.URL.Query().Get("idContainer"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainer":
			id := r.URL.Query().Get("idContainer")
			ct, ok := containers[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Container does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(ct)
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
resource "matomo_tagmanager_container" "test" {
  site_id = "3"
  context = "web"
  name    = "Main"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "id", "3/abc123"),
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "context", "web"),
				),
			},
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_container" "test" {
  site_id = "3"
  context = "web"
  name    = "Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "name", "Renamed"),
			},
		},
	})
}
