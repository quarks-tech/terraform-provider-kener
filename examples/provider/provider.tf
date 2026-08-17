terraform {
  required_providers {
    kener = {
      source = "quarks-tech/kener"
    }
  }
}

# Credentials can also be supplied via the KENER_ENDPOINT and KENER_API_TOKEN
# environment variables instead of these attributes.
provider "kener" {
  endpoint  = "https://status.example.com"
  api_token = var.kener_api_token # kener_... created in Settings -> API Keys
}

variable "kener_api_token" {
  type      = string
  sensitive = true
}
