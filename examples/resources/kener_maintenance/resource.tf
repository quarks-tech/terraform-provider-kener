resource "kener_monitor" "api" {
  tag          = "my-api"
  name         = "My API"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://api.example.com/health" })
}

resource "kener_maintenance" "weekly" {
  title            = "Weekly database maintenance"
  description      = "Routine Sunday maintenance window"
  start_date_time  = 1735732800 # Unix timestamp (seconds), minute-aligned
  rrule            = "FREQ=WEEKLY;BYDAY=SU"
  duration_seconds = 3600 # 1 hour
  status           = "ACTIVE"

  monitors = [
    {
      monitor_tag = kener_monitor.api.tag
      impact      = "MAINTENANCE"
    },
  ]
}
