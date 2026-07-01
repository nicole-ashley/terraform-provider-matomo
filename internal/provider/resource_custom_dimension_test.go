package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
			// Mirror real Matomo: id is a per-site sequential row id shared
			// across both scopes, while index is a per-scope slot number
			// that resets independently per scope. These are NOT the same
			// value except by coincidence for the very first dimension ever
			// created on a site.
			id := len(dims) + 1
			scope := r.URL.Query().Get("scope")
			maxIndex := 0
			for _, d := range dims {
				if d.Scope == scope && d.Index > maxIndex {
					maxIndex = d.Index
				}
			}
			index := maxIndex + 1
			dims[id] = &fakeDimension{
				ID: id, Index: index, Scope: scope,
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

// TestAccCustomDimensionResource_idAndIndexDiffer guards against a Critical
// bug where the resource used Matomo's per-scope "index" as if it were the
// per-site "id" (idDimension) required by
// CustomDimensions.configureExistingCustomDimension. A site with one
// visit-scope dimension (id=1, index=1) already has its "index" sequence
// reset for the "action" scope, so creating an action-scope dimension at
// index=1 gets a *different* real id (2). If Update/Delete pass "index"
// instead of the looked-up "id" to Matomo, they will silently mutate the
// wrong dimension (id=1, the visit-scope one) instead of the intended one
// (id=2, the action-scope one).
func TestAccCustomDimensionResource_idAndIndexDiffer(t *testing.T) {
	dims := map[int]*fakeDimension{
		1: {ID: 1, Index: 1, Scope: "visit", Name: "Pre-existing Visit Dim", Active: true},
	}
	srv := newCustomDimensionTestServer(t, dims)
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				// Create an action-scope dimension declared at index=1.
				// Matomo assigns it real id=2 (index sequence for "action"
				// is independent from "visit"), so index (1) coincides with
				// the existing visit-scope dimension's real id (1).
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_custom_dimension" "test" {
  site_id = "3"
  index   = 1
  scope   = "action"
  name    = "Action Dimension"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "id", "3/1"),
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "scope", "action"),
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "active", "true"),
				),
			},
			{
				// Update the action-scope dimension's name. This must
				// mutate dims[2] (real id=2), not dims[1] (the unrelated
				// pre-existing visit-scope dimension whose id happens to
				// equal the declared index).
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_custom_dimension" "test" {
  site_id = "3"
  index   = 1
  scope   = "action"
  name    = "Renamed Action Dimension"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "name", "Renamed Action Dimension"),
					checkFakeDimension(dims, 1, "Pre-existing Visit Dim", true),
					checkFakeDimension(dims, 2, "Renamed Action Dimension", true),
				),
			},
			{
				// Destroy the action-scope dimension (Matomo has no delete
				// API, so this deactivates it). Only dims[2] must be
				// affected; dims[1] must remain untouched and active.
				Config: testAccPreCheckConfig(srv),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkFakeDimension(dims, 1, "Pre-existing Visit Dim", true),
					checkFakeDimension(dims, 2, "Renamed Action Dimension", false),
				),
			},
		},
	})
}

// checkFakeDimension returns a TestCheckFunc that asserts the fake server's
// in-memory dimension record for the given id has the expected name and
// active state, without depending on any Terraform state attribute.
func checkFakeDimension(dims map[int]*fakeDimension, id int, wantName string, wantActive bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		d, ok := dims[id]
		if !ok {
			return fmt.Errorf("dims[%d] does not exist", id)
		}
		if d.Name != wantName {
			return fmt.Errorf("dims[%d].Name = %q, want %q", id, d.Name, wantName)
		}
		if d.Active != wantActive {
			return fmt.Errorf("dims[%d].Active = %v, want %v", id, d.Active, wantActive)
		}
		return nil
	}
}
