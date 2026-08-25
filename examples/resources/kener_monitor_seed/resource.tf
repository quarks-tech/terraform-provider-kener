# Seed the last 90 days of a monitor's history as UP, so a newly created monitor
# renders a green bar chart from day one instead of NO_DATA.
resource "kener_monitor" "api" {
  tag          = "my-api"
  name         = "My API"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://api.example.com/health" })
}

resource "kener_monitor_seed" "api" {
  monitor_tag = kener_monitor.api.tag
  # window_days defaults to 90, status to UP, latency/deviation to 0.
}

# A fuller example: a year of history with realistic-looking latency jitter.
resource "kener_monitor" "db" {
  tag          = "db-tcp"
  name         = "Database"
  monitor_type = "TCP"
  type_data    = jsonencode({ hosts = [{ type = "TCP", host = "db.example.com", port = 5432 }] })
}

resource "kener_monitor_seed" "db_year" {
  monitor_tag = kener_monitor.db.tag
  window_days = 365
  status      = "UP"
  latency     = 150
  deviation   = 30

  # Bump a triggers value to deliberately re-seed (overwrites the window).
  triggers = {
    version = "1"
  }
}
