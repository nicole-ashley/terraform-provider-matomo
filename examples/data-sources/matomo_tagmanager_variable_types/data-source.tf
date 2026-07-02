data "matomo_tagmanager_variable_types" "web" {
  context = "web"
}

output "available_variable_types" {
  value = [for t in data.matomo_tagmanager_variable_types.web.variable_types : t.id]
}
