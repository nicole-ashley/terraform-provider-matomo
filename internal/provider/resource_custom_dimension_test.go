package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type fakeDimension struct {
	ID, Index             int
	Scope, Name           string
	Active, CaseSensitive bool
}

func newCustomDimensionTestServer(t *testing.T, dims map[int]*fakeDimension) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "CustomDimensions.getConfiguredCustomDimensions":
			out := []map[string]any{}
			for _, d := range dims {
				out = append(out, map[string]any{
					"id": strconv.Itoa(d.ID), "index": strconv.Itoa(d.Index),
					"scope": d.Scope, "name": d.Name, "active": d.Active, "case_sensitive": d.CaseSensitive,
				})
			}
			_ = json.NewEncoder(w).Encode(out)
		case "CustomDimensions.configureNewCustomDimension":
			id := len(dims) + 1
			dims[id] = &fakeDimension{
				ID: id, Index: id, Scope: r.URL.Query().Get("scope"),
				Name: r.URL.Query().Get("name"), Active: r.URL.Query().Get("active") == "1",
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": strconv.Itoa(id)})
		case "CustomDimensions.configureExistingCustomDimension":
			id, _ := strconv.Atoi(r.URL.Query().Get("idDimension"))
			d, ok := dims[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "dimension not found"})
				return
			}
			d.Name = r.URL.Query().Get("name")
			d.Active = r.URL.Query().Get("active") == "1"
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
}

func TestAccCustomDimensionResource_createsNewSlot(t *testing.T) {
	dims := map[int]*fakeDimension{}
	srv := newCustomDimensionTestServer(t, dims)
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_custom_dimension" "test" {
  site_id = "3"
  index   = 1
  scope   = "visit"
  name    = "Test Dimension"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "id", "3/1"),
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "active", "true"),
				),
			},
		},
	})
}

func TestAccCustomDimensionResource_adoptsExistingSlot(t *testing.T) {
	dims := map[int]*fakeDimension{
		1: {ID: 1, Index: 1, Scope: "visit", Name: "Pre-existing", Active: true},
	}
	srv := newCustomDimensionTestServer(t, dims)
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_custom_dimension" "test" {
  site_id = "3"
  index   = 1
  scope   = "visit"
  name    = "Adopted Name"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_custom_dimension.test", "id", "3/1"),
			},
		},
	})

	if dims[1].Name != "Adopted Name" {
		t.Errorf("dims[1].Name = %q, want Adopted Name", dims[1].Name)
	}
}
