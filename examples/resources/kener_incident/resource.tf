resource "kener_monitor" "api" {
  tag          = "my-api"
  name         = "My API"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://api.example.com/health" })
}

resource "kener_incident" "outage" {
  title           = "API degraded performance"
  start_date_time = 1735732800 # Unix timestamp (seconds)

  monitors = [
    {
      monitor_tag = kener_monitor.api.tag
      impact      = "DEGRADED"
    },
  ]
}

# Progress updates on the incident, in order.
resource "kener_incident_comment" "investigating" {
  incident_id = kener_incident.outage.id
  comment     = "We are investigating elevated error rates."
  state       = "INVESTIGATING"
}

resource "kener_incident_comment" "resolved" {
  incident_id = kener_incident.outage.id
  comment     = "The issue has been resolved."
  state       = "RESOLVED"
}
