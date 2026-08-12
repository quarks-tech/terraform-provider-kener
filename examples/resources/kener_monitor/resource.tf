# A simple HTTP/API monitor.
resource "kener_monitor" "api" {
  tag          = "my-api"
  name         = "My API"
  description  = "Health endpoint for the public API"
  monitor_type = "API"
  cron         = "* * * * *"

  # type_data is a JSON object whose fields depend on monitor_type.
  type_data = jsonencode({
    url     = "https://api.example.com/health"
    method  = "GET"
    timeout = 10000
  })
}

# A TCP monitor with several hosts.
resource "kener_monitor" "tcp" {
  tag          = "db-tcp"
  name         = "Database TCP"
  monitor_type = "TCP"
  is_hidden    = true

  type_data = jsonencode({
    hosts = [
      { type = "IP4", host = "db1.internal", port = 5432, timeout = 1000 },
      { type = "IP4", host = "db2.internal", port = 5432, timeout = 1000 },
    ]
  })
}

# A group monitor that aggregates the two monitors above.
resource "kener_monitor" "group" {
  tag          = "platform"
  name         = "Platform"
  monitor_type = "GROUP"

  type_data = jsonencode({
    latencyCalculation = "AVG"
    executionDelay     = 1000
    monitors = [
      { tag = kener_monitor.api.tag, weight = 0.5 },
      { tag = kener_monitor.tcp.tag, weight = 0.5 },
    ]
  })
}
