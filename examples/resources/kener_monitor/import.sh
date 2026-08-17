# Monitors are imported by their tag.
# Note: type_data and monitor_settings_json are not recovered on import
# (Kener merges server-side defaults), so set them in your configuration after
# importing.
terraform import kener_monitor.api my-api
