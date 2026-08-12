data "kener_page" "home" {
  page_path = "~home"
}

output "home_monitors" {
  value = data.kener_page.home.monitors
}
