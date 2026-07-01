package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerVariableResource_basic(t *testing.T) {
	variables := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerVariable":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			variables[idStr] = map[string]any{
				"idvariable": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"parameters": map[string]any{}, "defaultValue": r.URL.Query().Get("defaultValue"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idvariable": idStr})
		case "TagManager.updateContainerVariable":
			id := r.URL.Query().Get("idVariable")
			variables[id]["name"] = r.URL.Query().Get("name")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerVariable":
			delete(variables, r.URL.Query().Get("idVariable"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerVariable":
			id := r.URL.Query().Get("idVariable")
			v, ok := variables[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Variable does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(v)
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
resource "matomo_tagmanager_variable" "test" {
  container_id  = "3/abc123"
  type          = "Constant"
  name          = "My var"
  default_value = "n/a"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "default_value", "n/a"),
				),
			},
		},
	})
}

// TestAccTagManagerVariableResource_multipleParameters exercises a variable
// with several parameter blocks. Variable.Parameters is a map[string]string
// in the Matomo client, just like Tag.Parameters and Trigger.Parameters.
// Tasks 16 and 17 both hit (16) or proactively avoided (17) the same bug:
// building state.Parameter by iterating the map directly, with no sort,
// produces a randomized list-nested-block order per process (Go map
// iteration order is not stable), which manifests as perpetual "the refresh
// plan was not empty" diffs even though nothing about the underlying data
// changed. This resource's Read() sorts parameters by name before returning
// them, so this test should pass deterministically across repeated runs
// (see -count=N invocations).
func TestAccTagManagerVariableResource_multipleParameters(t *testing.T) {
	variables := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerVariable":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			var params map[string]any
			_ = json.Unmarshal([]byte(r.URL.Query().Get("parameters")), &params)
			variables[idStr] = map[string]any{
				"idvariable": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"parameters": params, "defaultValue": r.URL.Query().Get("defaultValue"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idvariable": idStr})
		case "TagManager.updateContainerVariable":
			id := r.URL.Query().Get("idVariable")
			variables[id]["name"] = r.URL.Query().Get("name")
			var params map[string]any
			_ = json.Unmarshal([]byte(r.URL.Query().Get("parameters")), &params)
			variables[id]["parameters"] = params
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerVariable":
			delete(variables, r.URL.Query().Get("idVariable"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerVariable":
			id := r.URL.Query().Get("idVariable")
			v, ok := variables[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Variable does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(v)
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
resource "matomo_tagmanager_variable" "test" {
  container_id  = "3/abc123"
  type          = "Constant"
  name          = "My var"
  default_value = "n/a"
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
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "parameter.#", "4"),
				),
			},
		},
	})
}
