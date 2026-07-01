package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccTagManagerContainerResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Acceptance Site"
  urls = ["https://acc-container-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Test Container"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "name", "Acceptance Test Container"),
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "context", "web"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Acceptance Site"
  urls = ["https://acc-container-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Test Container Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "name", "Acceptance Test Container Renamed"),
			},
		},
	})
}

func TestAccTagManagerContainerResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Import Site"
  urls = ["https://acc-container-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Import Container"
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_container.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerContainerResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Disappears Site"
  urls = ["https://acc-container-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Disappears Container"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_container.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_container.test not found in state")
					}
					siteID, idContainer, err := parseContainerID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid container id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					return client.DeleteContainer(context.Background(), siteID, idContainer)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
