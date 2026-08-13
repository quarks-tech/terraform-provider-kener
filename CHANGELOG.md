# Changelog

All notable changes to this provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `tflog` debug logging in every resource's CRUD operations (visible under `TF_LOG=DEBUG`).
- Acceptance-test `CheckDestroy` for all resources, verifying resources are
  actually removed server-side after `terraform destroy` (and, for
  `kener_site_config`, that its value intentionally remains).
- Multi-monitor acceptance tests for `kener_incident`, `kener_maintenance` and
  `kener_page` that assert the configured monitor order is preserved.
- Unit tests for the model conversion helpers and `MonitorTags` JSON decoding.

### Fixed
- Possible "Provider produced inconsistent result after apply" when a resource
  had two or more monitors: `kener_incident`, `kener_maintenance` and
  `kener_page` now keep the configured `monitors` value/order instead of
  overwriting it with the (possibly reordered) server response. The server
  value is still used to seed the list on import.
- `kener_page` for the built-in `~home` page no longer calls the delete API on
  destroy; it drops the resource from state with a warning (the page cannot be
  deleted), mirroring `kener_site_config`.
- Page monitor tags are now ordered deterministically by their `position` when
  read from the server.
- `kener_site_config` keys whose `data_type` is `string` now detect drift: the
  value is refreshed from the server on read (object values remain configured
  verbatim because Kener deep-merges them server-side).

## [0.1.0] - Initial

- Initial provider with resources `kener_monitor`, `kener_page`,
  `kener_incident`, `kener_incident_comment`, `kener_maintenance` and
  `kener_site_config`, matching data sources, generated documentation and
  release tooling.
