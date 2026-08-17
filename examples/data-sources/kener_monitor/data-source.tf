data "kener_monitor" "api" {
  tag = "my-api"
}

output "api_monitor_status" {
  value = data.kener_monitor.api.status
}
