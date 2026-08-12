# Pages are imported by their page_path.
# Use ~home to import the built-in home page.
# Note: page_settings is not recovered on import (Kener merges server-side
# defaults), so set it in your configuration after importing.
terraform import kener_page.status status

# Import the built-in home page:
# terraform import kener_page.home '~home'
