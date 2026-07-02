package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerEnvironmentsDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_environments" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_environments.all", "id", "environments"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_environments.all", "environments.0.id", "live"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_environments.all", "environments.0.name", "Live"),
				),
			},
		},
	})
}
