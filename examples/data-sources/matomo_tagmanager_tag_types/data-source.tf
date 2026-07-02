data "matomo_tagmanager_tag_types" "web" {
  context = "web"
}

output "available_tag_types" {
  value = [for t in data.matomo_tagmanager_tag_types.web.tag_types : t.id]
}
