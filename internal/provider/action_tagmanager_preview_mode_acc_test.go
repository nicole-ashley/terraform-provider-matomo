package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnablePreviewModeAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated EnablePreviewMode Acceptance Site"
  urls = ["https://acc-enable-preview-mode.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated EnablePreviewMode Acceptance Container"
}

action "matomo_tagmanager_enable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.test.id
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_enable_preview_mode.preview]
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_tagmanager_container.test", "id"),
				),
			},
		},
	})
}

func TestAccDisablePreviewModeAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated DisablePreviewMode Acceptance Site"
  urls = ["https://acc-disable-preview-mode.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated DisablePreviewMode Acceptance Container"
}

action "matomo_tagmanager_disable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.test.id
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_disable_preview_mode.preview]
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_tagmanager_container.test", "id"),
				),
			},
		},
	})
}
