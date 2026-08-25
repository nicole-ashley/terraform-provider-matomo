package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Regression coverage for the two fields carrying
// trimTrailingNewlinePlanModifier (see trimTrailingNewlineOverrides in
// tools/gen/spec.go), configured the way the modifier exists for: an HCL
// heredoc, which always folds its own closing newline into the string
// value.
//
// Creating such a value used to fail outright with "Provider produced
// inconsistent result after apply" - Create reads the entity back from
// Matomo before setting state (see typedTagResource.Create's read-back
// comment), and Matomo doesn't retain the trailing newline, so the state
// it wrote could never match a plan that still carried one. The modifier
// only compared against prior state, of which a create has none, so
// nothing normalized the planned value and heredoc values could only be
// created by wrapping them in chomp() by hand.
//
// Each step's implicit post-apply plan check (TestStep defaults
// ExpectNonEmptyPlan to false) also re-covers the original perpetual-diff
// bug these fields were reported for.

func TestAccTagCustomhtml_heredocValueCreatesAndUpdates(t *testing.T) {
	testAccPreCheck(t)
	resourceName := "matomo_tagmanager_tag_customhtml.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagCustomhtmlHeredocConfig("<script>console.log('created');</script>"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					// The heredoc's own trailing newline is chomped, so
					// state matches what Matomo actually stored.
					resource.TestCheckResourceAttr(resourceName, "custom_html", "<script>console.log('created');</script>"),
				),
			},
			{
				Config: testAccTagCustomhtmlHeredocConfig("<script>console.log('updated');</script>"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "custom_html", "<script>console.log('updated');</script>"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccTagCustomhtmlHeredocConfig(body string) string {
	return `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Heredoc CustomHtml Acceptance Site"
  urls = ["https://acc-heredoc-customhtml.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Heredoc CustomHtml Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Heredoc CustomHtml Acceptance Trigger"
}

resource "matomo_tagmanager_tag_customhtml" "test" {
  container_id     = matomo_tagmanager_container.test.id
  name             = "heredoc-test-customhtml"
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]

  custom_html = <<EOT
` + body + `
EOT
}
`
}

func TestAccVariableCustomjsfunction_heredocValueCreatesAndUpdates(t *testing.T) {
	testAccPreCheck(t)
	resourceName := "matomo_tagmanager_variable_customjsfunction.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVariableCustomjsfunctionHeredocConfig("created"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "js_function", "function() {\n  return 'created';\n}"),
				),
			},
			{
				Config: testAccVariableCustomjsfunctionHeredocConfig("updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "js_function", "function() {\n  return 'updated';\n}"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccVariableCustomjsfunctionHeredocConfig(returnValue string) string {
	return `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Heredoc CustomJsFunction Acceptance Site"
  urls = ["https://acc-heredoc-customjsfunction.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Heredoc CustomJsFunction Acceptance Container"
}

resource "matomo_tagmanager_variable_customjsfunction" "test" {
  container_id = matomo_tagmanager_container.test.id
  name         = "heredoc-test-customjsfunction"

  // CustomJsFunctionVariable.php requires the value to start with the
  // literal word "function" (confirmed live and by reading the source).
  js_function = <<EOT
function() {
  return '` + returnValue + `';
}
EOT
}
`
}
