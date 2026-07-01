package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCustomDimensionResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Acceptance Site"
  urls = ["https://acc-dimension-test.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Acceptance Test Dimension"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "name", "Acceptance Test Dimension"),
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "active", "true"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Acceptance Site"
  urls = ["https://acc-dimension-test.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Acceptance Test Dimension Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_custom_dimension.test", "name", "Acceptance Test Dimension Renamed"),
			},
		},
	})
}

func TestAccCustomDimensionResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Import Site"
  urls = ["https://acc-dimension-import.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "action"
  name    = "Acceptance Import Dimension"
}
`,
			},
			{
				ResourceName:      "matomo_custom_dimension.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCustomDimensionResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Disappears Site"
  urls = ["https://acc-dimension-disappears.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Acceptance Disappears Dimension"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_custom_dimension.test"]
					if !ok {
						return fmt.Errorf("matomo_custom_dimension.test not found in state")
					}
					siteID, index, err := parseDimensionID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid custom dimension id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					dims, err := client.GetConfiguredCustomDimensions(ctx, siteID)
					if err != nil {
						return err
					}
					for _, d := range dims {
						if d.Index == index && d.Scope == "visit" {
							return client.ConfigureExistingCustomDimension(ctx, d.ID, siteID, d.Name, false)
						}
					}
					return fmt.Errorf("dimension at index %d not found for out-of-band deactivation", index)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccCustomDimensionResource_idIndexDivergence(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Dimension Divergence Site"
  urls = ["https://acc-dimension-divergence.example.com"]
}

resource "matomo_custom_dimension" "visit_dim" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Visit Dimension"
}

resource "matomo_custom_dimension" "action_dim" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "action"
  name    = "Action Dimension"
}
`,
				Check: func(s *terraform.State) error {
					visit, ok := s.RootModule().Resources["matomo_custom_dimension.visit_dim"]
					if !ok {
						return fmt.Errorf("matomo_custom_dimension.visit_dim not found in state")
					}
					action, ok := s.RootModule().Resources["matomo_custom_dimension.action_dim"]
					if !ok {
						return fmt.Errorf("matomo_custom_dimension.action_dim not found in state")
					}
					siteID, _, err := parseDimensionID(visit.Primary.ID)
					if err != nil {
						return err
					}
					client := testAccMatomoClient(t)
					dims, err := client.GetConfiguredCustomDimensions(context.Background(), siteID)
					if err != nil {
						return err
					}
					var visitID, actionID int
					for _, d := range dims {
						if d.Scope == "visit" && d.Index == 1 {
							visitID = d.ID
						}
						if d.Scope == "action" && d.Index == 1 {
							actionID = d.ID
						}
					}
					if visitID == 0 || actionID == 0 {
						return fmt.Errorf("could not find both dimensions via GetConfiguredCustomDimensions: visitID=%d actionID=%d", visitID, actionID)
					}
					if visitID == actionID {
						return fmt.Errorf("expected visit and action dimension ids to diverge (both index=1, different scopes), got same id=%d for both — Matomo's id/index behavior may not match internal/matomo/customdimensions.go's documented assumption, re-verify that comment", visitID)
					}
					t.Logf("confirmed id/index divergence: visit dimension id=%d index=1, action dimension id=%d index=1", visitID, actionID)
					_ = action // referenced only via Primary.ID above; keep for clarity that both resources are checked
					return nil
				},
			},
		},
	})
}
