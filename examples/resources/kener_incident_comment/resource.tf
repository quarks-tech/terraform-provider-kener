resource "kener_incident_comment" "update" {
  incident_id = kener_incident.outage.id
  comment     = "We have identified the root cause."
  state       = "IDENTIFIED"
}
