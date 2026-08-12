resource "kener_monitor" "api" {
  tag           = "my-api"
  name          = "My API"
  monitor_type  = "API"
  category_name = "Core"
  type_data     = jsonencode({ url = "https://api.example.com/health" })
}

resource "kener_page" "status" {
  page_path      = "status"
  page_title     = "Example Status"
  page_header    = "Example Service Status"
  page_subheader = "Current status of our services"

  # Ordered list of monitor tags to show on the page.
  monitors = [kener_monitor.api.tag]

  page_settings = jsonencode({
    monitor_layout_style = "default-list"
    monitor_status_history_days = {
      desktop = 90
      mobile  = 30
    }
  })
}
