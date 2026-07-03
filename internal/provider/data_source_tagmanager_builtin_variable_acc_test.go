package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerBuiltinVariableDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_builtin_variable" "this" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "id", "builtin"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "page_path", "PagePath"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "page_hostname", "PageHostname"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "click_id", "ClickId"),
				),
			},
		},
	})
}
