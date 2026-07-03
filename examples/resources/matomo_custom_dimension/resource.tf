resource "matomo_custom_dimension" "user_type" {
  site_id = matomo_site.main.id
  name    = "User Type"
  scope   = "visit"
  index   = 1
  active  = true
}
