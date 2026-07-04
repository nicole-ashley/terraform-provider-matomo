package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccTagManagerVariableResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Acceptance Site"
  urls = ["https://acc-variable-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Acceptance Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Test Variable"
  default_value = "acceptance-default"
  parameter {
    name  = "constantValue"
    value = "acceptance-constant-value"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "name", "Acceptance Test Variable"),
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "default_value", "acceptance-default"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Acceptance Site"
  urls = ["https://acc-variable-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Acceptance Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Test Variable Renamed"
  default_value = "acceptance-default"
  parameter {
    name  = "constantValue"
    value = "acceptance-constant-value"
  }
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "name", "Acceptance Test Variable Renamed"),
			},
		},
	})
}

func TestAccTagManagerVariableResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Import Site"
  urls = ["https://acc-variable-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Import Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Import Variable"
  default_value = "n/a"
  parameter {
    name  = "constantValue"
    value = "acceptance-constant-value"
  }
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_variable.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerVariableResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Disappears Site"
  urls = ["https://acc-variable-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Disappears Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Disappears Variable"
  default_value = "n/a"
  parameter {
    name  = "constantValue"
    value = "acceptance-constant-value"
  }
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_variable.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_variable.test not found in state")
					}
					siteID, idContainer, idVariable, err := parseEntityID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid variable id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					versionID, err := resolveDraftVersionID(ctx, client, siteID, idContainer)
					if err != nil {
						return err
					}
					return client.DeleteContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccTagManagerVariableResource_parameterList(t *testing.T) {
	testAccPreCheck(t)
	resourceName := "matomo_tagmanager_variable.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generic Variable ParameterList Acceptance Site"
  urls = ["https://acc-generic-var-paramlist-test.example.com"]
}

resource "matomo_custom_dimension" "cat" {
  site_id = matomo_site.test.id
  name    = "Page Category"
  scope   = "action"
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generic Variable ParameterList Acceptance Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "MatomoConfiguration"
  name         = "acceptance-test-paramlist"

  parameter {
    name  = "matomoUrl"
    value = "https://acc-generic-var-paramlist-test.example.com"
  }
  parameter {
    name  = "idSite"
    value = matomo_site.test.id
  }
  parameter_list {
    name = "customDimensions"
    row {
      item {
        key   = "index"
        value = matomo_custom_dimension.cat.index
      }
      item {
        key   = "value"
        value = "{{Page Category}}"
      }
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "parameter_list.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameter_list.0.name", "customDimensions"),
					resource.TestCheckResourceAttr(resourceName, "parameter_list.0.row.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameter_list.0.row.0.item.#", "2"),
				),
			},
		},
	})
}
