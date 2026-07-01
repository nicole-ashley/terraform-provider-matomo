package provider

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSiteResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Test Site"
  urls = ["https://acc-test.example.com"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_site.test", "name", "Acceptance Test Site"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.0", "https://acc-test.example.com"),
					resource.TestCheckResourceAttrSet("matomo_site.test", "id"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Test Site Renamed"
  urls = ["https://acc-test.example.com"]
}
`,
				Check: resource.TestCheckResourceAttr("matomo_site.test", "name", "Acceptance Test Site Renamed"),
			},
		},
	})
}

func TestAccSiteResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Import Test Site"
  urls = ["https://acc-import-test.example.com"]
}
`,
			},
			{
				ResourceName:      "matomo_site.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSiteResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Disappears Test Site"
  urls = ["https://acc-disappears-test.example.com"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_site.test", "id"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["matomo_site.test"]
						if !ok {
							return fmt.Errorf("matomo_site.test not found in state")
						}
						idSite, err := strconv.Atoi(rs.Primary.ID)
						if err != nil {
							return fmt.Errorf("invalid site id %q: %w", rs.Primary.ID, err)
						}
						client := testAccMatomoClient(t)
						return client.DeleteSite(context.Background(), idSite)
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
