data "kener_maintenance" "weekly" {
  id = "5"
}

output "maintenance_url" {
  value = data.kener_maintenance.weekly.url
}
