package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCreateContainerVersionAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated CreateContainerVersion Acceptance Site"
  urls = ["https://acc-create-container-version.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated CreateContainerVersion Acceptance Container"
}

action "matomo_tagmanager_create_container_version" "release" {
  config {
    container_id = matomo_tagmanager_container.test.id
    name         = "acceptance-test-version"
    description  = "created by an acceptance test"
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_create_container_version.release]
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
