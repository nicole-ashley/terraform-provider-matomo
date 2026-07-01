package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSmoke_ProviderReachesRealMatomo is the first real-Matomo test in
// this codebase. It exists to prove the docker-compose fixture, bootstrap
// script, and acceptance.yml workflow reach a genuinely running Matomo
// instance end-to-end, before any resource-specific acceptance tests are
// added in later tasks. It creates a site directly via HCL and reads it
// back via the data source — the simplest possible real round trip.
func TestAccSmoke_ProviderReachesRealMatomo(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "smoke" {
  name = "Acceptance Smoke Test"
  urls = ["https://smoke-test.example.com"]
}

data "matomo_site" "smoke" {
  id = matomo_site.smoke.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_site.smoke", "name", "Acceptance Smoke Test"),
					resource.TestCheckResourceAttrPair("data.matomo_site.smoke", "id", "matomo_site.smoke", "id"),
				),
			},
		},
	})
}
