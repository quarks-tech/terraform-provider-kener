# Terraform Provider for Kener

A [Terraform](https://developer.hashicorp.com/terraform) provider for
[Kener](https://kener.ing), the open-source status-page tool. It manages Kener
resources through the instance's `/api/v4` REST API.

Built on the modern [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework).

## Status

This provider is being built incrementally. Implemented so far:

| Resource / data source | Status |
| --- | --- |
| `kener_monitor` (resource + data source) | ✅ Create / Read / Update / Delete / Import |
| `kener_page` (resource + data source) | ✅ Create / Read / Update / Delete / Import |
| `kener_incident` (resource + data source) | ✅ Create / Read / Update / Delete / Import |
| `kener_incident_comment` | ✅ Create / Read / Update / Delete / Import |
| `kener_maintenance` (resource + data source) | ✅ Create / Read / Update / Delete / Import |
| `kener_site_config` (resource + data source) | ✅ Create* / Read / Update / Delete* / Import |

<sub>\* Site-config keys are a fixed set that can only be updated: "create" sets the key's value, and "delete" just removes it from Terraform state (the value stays on the server).</sub>

See [`docs/research/kener-api.md`](docs/research/kener-api.md) for the full API
surface map and notes on which Kener features are (and are not) manageable via
the API.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.25 (to build the provider)
- A Kener instance and an API token (created in Kener under **Settings → API Keys**)

## Using the provider

```hcl
terraform {
  required_providers {
    kener = {
      source = "quarks-tech/kener"
    }
  }
}

provider "kener" {
  endpoint  = "https://status.example.com" # or KENER_ENDPOINT
  api_token = var.kener_api_token          # or KENER_API_TOKEN (sensitive)
}

resource "kener_monitor" "api" {
  tag          = "my-api"
  name         = "My API"
  monitor_type = "API"
  type_data = jsonencode({
    url = "https://api.example.com/health"
  })
}
```

The `endpoint` and `api_token` can also be supplied via the `KENER_ENDPOINT` and
`KENER_API_TOKEN` environment variables.

### About `type_data`

A monitor's type-specific configuration is expressed as a JSON object via the
`type_data` attribute (use `jsonencode(...)`). The available fields depend on
`monitor_type` (`API`, `TCP`, `DNS`, `SSL`, `SQL`, `GROUP`, …) — see the
[Kener monitor docs](https://kener.ing/docs). Kener merges its own server-side
defaults into this value; the provider stores exactly what you configure, so no
perpetual diff appears. For the same reason, `type_data` is **not** recovered on
`terraform import` — set it in configuration after importing.

## Development

```bash
make build      # compile the provider
make test       # unit tests (client is httptest-based; no live Kener needed)
make docs       # regenerate docs/ from the schema + examples
make lint       # golangci-lint
make fmt        # gofmt + terraform fmt
```

### Acceptance tests

Acceptance tests run against a real Kener instance in local Docker. The target
starts Kener, mints an API token by inserting it directly into the instance's
database (bypassing the admin UI), and runs the tests:

```bash
make testacc        # starts Kener, runs TF_ACC tests
make kener-down     # tear down the local Kener instance
```

To point the tests at your own instance instead:

```bash
TF_ACC=1 KENER_ENDPOINT=https://status.example.com KENER_API_TOKEN=kener_... \
  go test ./internal/provider/... -v
```

## Architecture

```
internal/
  client/     standalone, typed Kener API client (no Terraform deps; httptest-tested)
  provider/   provider config + one file per resource / data source
main.go       provider entrypoint (providerserver.Serve)
```

The `internal/client` package is deliberately free of any Terraform imports so it
can be tested and reused in isolation.

## License

[MPL-2.0](LICENSE).
