resource "matomo_tagmanager_trigger" "custom" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "Custom Page View Trigger"
}

# condition.variable accepts either a bare Matomo built-in variable
# identifier, or a reference to the matomo_tagmanager_builtin_variable data
# source for editor autocomplete on the known built-in names (this is a
# discoverability aid only - both forms are wire-identical, and neither is
# an exhaustive list of every value Matomo accepts, since third-party
# plugins can contribute more and any matomo_tagmanager_variable* resource
# is referenceable via a {{Name}} macro regardless).
data "matomo_tagmanager_builtin_variable" "this" {}

resource "matomo_tagmanager_trigger" "checkout" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "Checkout Page"

  condition {
    comparison = "equals"
    variable   = data.matomo_tagmanager_builtin_variable.this.page_path
    value      = "/checkout"
  }
}
