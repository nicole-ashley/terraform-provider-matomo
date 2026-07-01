package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestUnitSiteDataSource_byName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("method") != "SitesManager.getAllSites" {
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idsite": 1, "name": "Other", "timezone": "UTC", "currency": "USD"},
			{"idsite": 2, "name": "Example", "timezone": "UTC", "currency": "USD"},
		})
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
data "matomo_site" "test" {
  name = "Example"
}
`,
				Check: resource.TestCheckResourceAttr("data.matomo_site.test", "id", "2"),
			},
		},
	})
}
