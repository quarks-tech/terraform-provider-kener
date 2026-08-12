# A string-valued key.
resource "kener_site_config" "site_name" {
  key   = "siteName"
  value = jsonencode("Example Status")
}

# An object-valued key (navigation links).
resource "kener_site_config" "nav" {
  key = "nav"
  value = jsonencode([
    { name = "Docs", url = "https://example.com/docs", iconURL = "" },
    { name = "Home", url = "https://example.com", iconURL = "" },
  ])
}
