package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTriggerTypesDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_trigger_types" "web" {
  context = "web"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_trigger_types.web", "id", "web"),
					resource.TestCheckResourceAttrSet("data.matomo_tagmanager_trigger_types.web", "trigger_types.#"),
					resource.TestCheckTypeSetElemNestedAttrs("data.matomo_tagmanager_trigger_types.web", "trigger_types.*", map[string]string{
						"id": "PageView",
					}),
				),
			},
		},
	})
}
