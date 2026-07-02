data "matomo_tagmanager_trigger_types" "web" {
  context = "web"
}

output "available_trigger_types" {
  value = [for t in data.matomo_tagmanager_trigger_types.web.trigger_types : t.id]
}
