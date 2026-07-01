package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestUnitTagManagerTriggerResource_basic(t *testing.T) {
	triggers := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerTrigger":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			triggers[idStr] = map[string]any{
				"idtrigger": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"parameters": map[string]any{},
				"conditions": []map[string]any{{"comparison": "equals", "actual": "url_path", "value": "/checkout"}},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idtrigger": idStr})
		case "TagManager.updateContainerTrigger":
			id := r.URL.Query().Get("idTrigger")
			triggers[id]["name"] = r.URL.Query().Get("name")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerTrigger":
			delete(triggers, r.URL.Query().Get("idTrigger"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerTrigger":
			id := r.URL.Query().Get("idTrigger")
			trig, ok := triggers[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Trigger does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(trig)
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
resource "matomo_tagmanager_trigger" "test" {
  container_id = "3/abc123"
  type         = "PageView"
  name         = "Checkout page"
  condition {
    comparison = "equals"
    actual     = "url_path"
    value      = "/checkout"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.comparison", "equals"),
				),
			},
		},
	})
}

// TestAccTagManagerTriggerResource_multipleParameters exercises a trigger
// with several parameter blocks. Trigger.Parameters is a map[string]string
// in the Matomo client (unlike Trigger.Conditions, which is a real ordered
// []Condition slice from the JSON array response and needs no sorting).
// Task 16's matomo_tagmanager_tag resource had this exact bug: Read() built
// state.Parameter by iterating the parameters map directly with no sort,
// and since Go randomizes map iteration order per process, the resulting
// list-nested-block order varied from run to run, producing "the refresh
// plan was not empty" failures on later plans even though nothing about the
// underlying data had changed. This resource's Read() sorts the parameters
// by name before returning them from the start, so this test should pass
// deterministically across repeated runs (see -count=N invocations).
func TestUnitTagManagerTriggerResource_multipleParameters(t *testing.T) {
	triggers := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerTrigger":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			var params map[string]any
			_ = json.Unmarshal([]byte(r.URL.Query().Get("parameters")), &params)
			triggers[idStr] = map[string]any{
				"idtrigger": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"parameters": params,
				"conditions": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idtrigger": idStr})
		case "TagManager.updateContainerTrigger":
			id := r.URL.Query().Get("idTrigger")
			triggers[id]["name"] = r.URL.Query().Get("name")
			var params map[string]any
			_ = json.Unmarshal([]byte(r.URL.Query().Get("parameters")), &params)
			triggers[id]["parameters"] = params
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerTrigger":
			delete(triggers, r.URL.Query().Get("idTrigger"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerTrigger":
			id := r.URL.Query().Get("idTrigger")
			trig, ok := triggers[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Trigger does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(trig)
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
resource "matomo_tagmanager_trigger" "test" {
  container_id = "3/abc123"
  type         = "PageView"
  name         = "Checkout page"
  parameter {
    name  = "alpha"
    value = "1"
  }
  parameter {
    name  = "bravo"
    value = "2"
  }
  parameter {
    name  = "charlie"
    value = "3"
  }
  parameter {
    name  = "delta"
    value = "4"
  }
  parameter {
    name  = "echo"
    value = "5"
  }
  parameter {
    name  = "foxtrot"
    value = "6"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "parameter.#", "6"),
				),
			},
		},
	})
}
