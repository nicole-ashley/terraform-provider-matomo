resource "matomo_tagmanager_container" "main" {
  site_id     = matomo_site.main.id
  context     = "web"
  name        = "Main Website Container"
  description = "Primary Tag Manager container for the main website"
}
