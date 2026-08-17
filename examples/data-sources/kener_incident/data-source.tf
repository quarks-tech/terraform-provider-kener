data "kener_incident" "outage" {
  id = "42"
}

output "outage_state" {
  value = data.kener_incident.outage.state
}

# All status updates on the incident, in order.
output "outage_comments" {
  value = data.kener_incident.outage.comments
}
