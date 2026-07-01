package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccTagManagerTagResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Acceptance Site"
  urls = ["https://acc-tag-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Tag Acceptance Trigger"
}

resource "matomo_tagmanager_tag" "test" {
  container_id     = matomo_tagmanager_container.test.id
  type             = "CustomHtml"
  name             = "Acceptance Test Tag"
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]
  parameter {
    name  = "customHtml"
    value = "<script>console.log('acceptance test')</script>"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "name", "Acceptance Test Tag"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "active"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "parameter.0.name", "customHtml"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "fire_trigger_ids.#", "1"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Acceptance Site"
  urls = ["https://acc-tag-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Tag Acceptance Trigger"
}

resource "matomo_tagmanager_tag" "test" {
  container_id     = matomo_tagmanager_container.test.id
  type             = "CustomHtml"
  name             = "Acceptance Test Tag"
  status           = "paused"
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]
  parameter {
    name  = "customHtml"
    value = "<script>console.log('acceptance test')</script>"
  }
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "paused"),
			},
		},
	})
}

func TestAccTagManagerTagResource_multipleParameters(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Multi-Param Acceptance Site"
  urls = ["https://acc-tag-multiparam-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Multi-Param Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Tag Multi-Param Acceptance Trigger"
}

resource "matomo_tagmanager_tag" "test" {
  container_id     = matomo_tagmanager_container.test.id
  type             = "CustomHtml"
  name             = "Acceptance Multi-Param Tag"
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]
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
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "parameter.#", "4"),
			},
		},
	})
}

func TestAccTagManagerTagResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Import Site"
  urls = ["https://acc-tag-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Import Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Tag Import Trigger"
}

resource "matomo_tagmanager_tag" "test" {
  container_id     = matomo_tagmanager_container.test.id
  type             = "CustomHtml"
  name             = "Acceptance Import Tag"
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_tag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerTagResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Disappears Site"
  urls = ["https://acc-tag-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Disappears Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Tag Disappears Trigger"
}

resource "matomo_tagmanager_tag" "test" {
  container_id     = matomo_tagmanager_container.test.id
  type             = "CustomHtml"
  name             = "Acceptance Disappears Tag"
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_tag.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_tag.test not found in state")
					}
					siteID, idContainer, idTag, err := parseEntityID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid tag id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					versionID, err := resolveDraftVersionID(ctx, client, siteID, idContainer)
					if err != nil {
						return err
					}
					return client.DeleteContainerTag(ctx, siteID, idContainer, versionID, idTag)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
