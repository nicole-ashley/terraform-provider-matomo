package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Regression coverage for the two fields carrying chompedStringType (see
// trimTrailingNewlineOverrides in tools/gen/spec.go), configured the way
// that type exists for: an HCL heredoc, which always folds its own closing
// newline into the string value.
//
// Creating such a value used to fail outright with "Provider produced
// inconsistent result after apply" - Create reads the entity back from
// Matomo before setting state (see typedTagResource.Create's read-back
// comment), and Matomo doesn't retain the trailing newline, so the state
// it wrote could never match a plan that still carried one.
//
// State ends up holding the configured value verbatim, newline and all:
// semantic equality resolves the mismatch by keeping the prior (planned)
// value rather than by rewriting the plan, which is the only thing
// Terraform permits for an attribute that isn't Computed.
//
// Each step's implicit post-apply plan check (TestStep defaults
// ExpectNonEmptyPlan to false) also covers the perpetual-diff bug these
// fields were originally reported for.

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
					resource.TestCheckResourceAttr(resourceName, "custom_html", "<script>console.log('created');</script>\n"),
				),
			},
			{
				Config: testAccTagCustomhtmlHeredocConfig("<script>console.log('updated');</script>"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "custom_html", "<script>console.log('updated');</script>\n"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				// An import has no prior value to hold on to, so
				// custom_html lands in state exactly as Matomo stores it -
				// chomped - where the applied state above kept the
				// heredoc's newline. That difference is benign and
				// self-correcting: on the next plan
				// trimTrailingNewlinePlanModifier sees the two agree
				// modulo the newline and plans the prior state, so an
				// imported resource reports no diff. Nothing else about
				// the imported state is exempt.
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"custom_html"},
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
					resource.TestCheckResourceAttr(resourceName, "js_function", "function() {\n  return 'created';\n}\n"),
				),
			},
			{
				Config: testAccVariableCustomjsfunctionHeredocConfig("updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "js_function", "function() {\n  return 'updated';\n}\n"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				// See the customhtml import step above for why js_function
				// is the one attribute exempt from ImportStateVerify here.
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"js_function"},
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
