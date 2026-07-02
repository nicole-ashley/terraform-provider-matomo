// internal/provider/data_source_tagmanager_tag_types_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTagTypesDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_tag_types" "web" {
  context = "web"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_tag_types.web", "id", "web"),
					resource.TestCheckResourceAttrSet("data.matomo_tagmanager_tag_types.web", "tag_types.#"),
					resource.TestCheckTypeSetElemNestedAttrs("data.matomo_tagmanager_tag_types.web", "tag_types.*", map[string]string{
						"id": "CustomHtml",
					}),
				),
			},
		},
	})
}
