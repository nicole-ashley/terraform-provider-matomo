package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newTestProviderFactories(srv *httptest.Server) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"matomo": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func testAccPreCheckConfig(srv *httptest.Server) string {
	return `
provider "matomo" {
  base_url  = "` + srv.URL + `"
  api_token = "test-token"
}
`
}

func TestUnitSiteResource_basic(t *testing.T) {
	sites := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		method := r.URL.Query().Get("method")
		switch method {
		case "SitesManager.addSite":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			sites[idStr] = map[string]any{
				"idsite": idStr, "name": r.URL.Query().Get("siteName"),
				"urls":     []string(r.URL.Query()["urls[]"]),
				"timezone": "UTC", "currency": "USD",
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"value": idStr})
		case "SitesManager.updateSite":
			id := r.URL.Query().Get("idSite")
			sites[id]["name"] = r.URL.Query().Get("siteName")
			sites[id]["urls"] = []string(r.URL.Query()["urls[]"])
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "SitesManager.deleteSite":
			delete(sites, r.URL.Query().Get("idSite"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "SitesManager.getSiteFromId":
			id := r.URL.Query().Get("idSite")
			site, ok := sites[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Website id Not found"})
				return
			}
			_ = json.NewEncoder(w).Encode(site)
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_site" "test" {
  name = "Example"
  urls = ["https://example.com"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_site.test", "name", "Example"),
					resource.TestCheckResourceAttrSet("matomo_site.test", "id"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.#", "1"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.0", "https://example.com"),
				),
			},
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_site" "test" {
  name = "Renamed"
  urls = ["https://example.com", "https://example.org"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_site.test", "name", "Renamed"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.#", "2"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.0", "https://example.com"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.1", "https://example.org"),
				),
			},
		},
	})
}
