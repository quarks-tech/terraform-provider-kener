data "kener_site_config" "title" {
  key = "title"
}

output "site_title" {
  value = jsondecode(data.kener_site_config.title.value)
}
