data "matomo_tagmanager_contexts" "all" {}

output "available_contexts" {
  value = [for c in data.matomo_tagmanager_contexts.all.contexts : c.id]
}
