package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerContextsDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_contexts" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_contexts.all", "id", "contexts"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_contexts.all", "contexts.0.id", "web"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_contexts.all", "contexts.0.name", "Web"),
				),
			},
		},
	})
}
