package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccTagManagerTriggerResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Acceptance Site"
  urls = ["https://acc-trigger-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Test Trigger"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "name", "Acceptance Test Trigger"),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Acceptance Site"
  urls = ["https://acc-trigger-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Test Trigger Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "name", "Acceptance Test Trigger Renamed"),
			},
		},
	})
}

func TestAccTagManagerTriggerResource_withConditions(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Conditions Acceptance Site"
  urls = ["https://acc-trigger-conditions-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Conditions Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Conditions Trigger"
  condition {
    comparison = "equals"
    actual     = "url_path"
    value      = "/checkout"
  }
  condition {
    comparison = "contains"
    actual     = "url_domain"
    value      = "example.com"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.#", "2"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.comparison", "equals"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.actual", "url_path"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.value", "/checkout"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.1.comparison", "contains"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.1.actual", "url_domain"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.1.value", "example.com"),
				),
			},
		},
	})
}

func TestAccTagManagerTriggerResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Import Site"
  urls = ["https://acc-trigger-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Import Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Import Trigger"
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_trigger.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerTriggerResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Disappears Site"
  urls = ["https://acc-trigger-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Disappears Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Disappears Trigger"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_trigger.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_trigger.test not found in state")
					}
					siteID, idContainer, idTrigger, err := parseEntityID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid trigger id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					versionID, err := resolveDraftVersionID(ctx, client, siteID, idContainer)
					if err != nil {
						return err
					}
					return client.DeleteContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
