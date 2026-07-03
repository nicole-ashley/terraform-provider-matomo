package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Exercises condition support on a single generated trigger type
// (CustomEvent), proving typed_trigger_resource.go's shared Schema()
// injection and Create/Update/Read wiring work end-to-end against a real
// Matomo instance - conditions are shared runtime, not per-type generated
// code, so one type is sufficient coverage (see typed_trigger_resource.go's
// Schema() doc comment).
func TestAccTypedTriggerConditions_customevent(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Typed Trigger Conditions Acceptance Site"
  urls = ["https://acc-typed-trigger-conditions-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Typed Trigger Conditions Acceptance Container"
}

resource "matomo_tagmanager_trigger_customevent" "test" {
  container_id = matomo_tagmanager_container.test.id
  name         = "Acceptance Conditions Trigger"
  event_name   = "add_to_cart"
  condition {
    comparison = "equals"
    variable   = "PagePath"
    value      = "/checkout"
  }
  condition {
    comparison = "contains"
    variable   = "PageHostname"
    value      = "example.com"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.#", "2"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.0.comparison", "equals"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.0.variable", "PagePath"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.0.value", "/checkout"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.1.comparison", "contains"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.1.variable", "PageHostname"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.1.value", "example.com"),
				),
			},
			{
				ResourceName:      "matomo_tagmanager_trigger_customevent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
