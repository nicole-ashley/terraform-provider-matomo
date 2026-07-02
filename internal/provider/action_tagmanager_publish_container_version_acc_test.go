package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPublishContainerVersionAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated PublishContainerVersion Acceptance Site"
  urls = ["https://acc-publish-container-version.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated PublishContainerVersion Acceptance Container"
}

action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_id = matomo_tagmanager_container.test.id
    environment  = "live"
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_publish_container_version.go_live]
    }
  }
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
					versions, err := client.GetContainerVersions(context.Background(), siteID, idContainer)
					if err != nil {
						return fmt.Errorf("listing container versions: %w", err)
					}
					for _, v := range versions {
						if strings.HasPrefix(v.Name, "terraform-release-") {
							return nil
						}
					}
					return fmt.Errorf("no container version with a %q-prefixed name found after publish_container_version action ran; versions = %+v", "terraform-release-", versions)
				},
			},
		},
	})
}
