# Kener HTTP API — Terraform Provider Surface Map

Researched against the live source on `main`. The authoritative contract is the
bundled OpenAPI 3.1 spec (`static/api-references/v4.json`), cross-checked against
the actual SvelteKit route handlers and the docs. Where docs and source disagree,
**source wins** and it is flagged.

## 0. Conventions, auth, and global behavior

**Base path / versioning.** All manageable endpoints live under **`/api/v4/`**
(e.g. `GET https://your-kener.com/api/v4/pages`). This is the only implemented API
version. Always use `/api/v4`.

**Authentication** (verified in `src/hooks.server.ts` + `apiController.ts`):
- Header: **`Authorization: Bearer <API_KEY>`** (case-insensitive `bearer`, exactly
  two space-separated parts). No custom header, no query-param auth.
- The key is a single **opaque token**: `generateApiKey()` returns
  `"kener_" + 32 random bytes as hex` → format **`kener_<64 hex chars>`**.
- Verification (`VerifyAPIKey`): the token is hashed and looked up; valid iff a row
  exists **and** `status == "ACTIVE"`. **There are NO permission scopes** — a valid
  key can call every endpoint.
- The only unauthenticated path is **`/api/status`**. Everything else returns
  `401 {"error":{"code":"UNAUTHORIZED",...}}` without a valid key.
- Missing/malformed identifier is caught in middleware → `404
  {"error":{"code":"NOT_FOUND","message":"... not found"}}` before the handler runs.

**API keys are created only in the admin UI** (`/manage` → Settings → API Keys).
There is **no `/api/v4` endpoint for API keys**, so the provider cannot manage keys —
it must be configured with a pre-existing key.

**Error envelope (real):** `{"error":{"code":"<STRING>","message":"<STRING>"}}`.
Codes: `UNAUTHORIZED`, `NOT_FOUND`, `BAD_REQUEST`, `INTERNAL_ERROR`. (Docs show a
`success` field that does **not** exist in real responses.)

**Pagination:** Only maintenance events paginate (`page` default `1`, `limit`
default `20`, with `total`). Every other collection returns the full array.

**Rate limiting:** **None in the code** (the auth doc's 100 req/min claim is stale).

**Doc/source inconsistencies:**
- `authentication.md` documents `/api/monitors`, `/api/me`, permission scopes, key
  expiry, rate limiting, `success` envelope — **none exist in v4 source** (legacy v3).
- OpenAPI omits `confirmation_threshold`, but POST/PATCH handlers accept/validate it
  (integer 1–60).
- OpenAPI types several boolean-ish monitor fields (`is_hidden`,
  `include_degraded_in_downtime`) as `string` — they are stored as `"YES"`/`"NO"`.

---

## 1. Endpoint inventory

| Resource | Method + Path | Notes |
|---|---|---|
| **Monitors** | `GET /api/v4/monitors` | filters: `status`, `category_name`, `monitor_type`, `is_hidden` |
| | `POST /api/v4/monitors` | create; 201 |
| | `GET /api/v4/monitors/{monitor_tag}` | |
| | `PATCH /api/v4/monitors/{monitor_tag}` | partial update; tag immutable |
| | `DELETE /api/v4/monitors/{monitor_tag}` | cascades |
| **Monitor Data** | `GET/PATCH /api/v4/monitors/{monitor_tag}/data` | runtime telemetry (not config) |
| | `GET/PATCH /api/v4/monitors/{monitor_tag}/data/{timestamp}` | single point |
| **Incidents** | `GET /api/v4/incidents` | filters: `start_ts`, `end_ts`, `monitor_tags` (CSV) |
| | `POST /api/v4/incidents` | 201 |
| | `GET/PATCH/DELETE /api/v4/incidents/{incident_id}` | |
| **Incident Comments** | `GET/POST /api/v4/incidents/{incident_id}/comments` | |
| | `GET/PATCH/DELETE /api/v4/incidents/{incident_id}/comments/{comment_id}` | |
| **Pages** | `GET /api/v4/pages` · `POST /api/v4/pages` | 201 |
| | `GET/PATCH/DELETE /api/v4/pages/{page_path}` | home addressed as `~home` |
| **Site** (global config) | `GET /api/v4/site` · `GET /api/v4/site/{config_key}` | |
| | `PATCH /api/v4/site/{config_key}` | **update only** — fixed key set |
| **Maintenances** | `GET /api/v4/maintenances` · `POST /api/v4/maintenances` | 201 |
| | `GET/PATCH/DELETE /api/v4/maintenances/{maintenance_id}` | |
| **Maintenance Events** | `GET .../maintenances/events` (paginated) | **no POST** — generated from `rrule` |
| | `GET/PATCH/DELETE /api/v4/maintenances/{maintenance_id}/events/{event_id}` | |

**Creatable vs read-only via API:**
- Full CRUD: **monitors, incidents, incident comments, pages, maintenances**.
- No create: maintenance **events** (rrule-generated), **site** config (fixed keys).
- Runtime telemetry (not config): monitor data points.
- **Not exposed at all**: API keys, users, triggers/alert-configs,
  subscribers/subscriptions. They exist as DB entities + `/manage` dashboard
  endpoints but have **no `/api/v4` surface**.

**Success envelopes:** collections wrap in a named array key, singletons in a named
object key — monitors → `{monitors:[...]}` / `{monitor:{...}}`; incidents →
`{incidents}`/`{incident}` (+ `{comments}`/`{comment}`); pages → `{pages}`/`{page}`;
site → `{site_data:[...]}` (list) or a bare `SiteDataEntry`; maintenances →
`{maintenances}`/`{maintenance}` (+ `{events,page,limit,total}`/`{event}`). All
DELETEs → `{message: "... deleted successfully"}`.

---

## 2. Monitor (reference resource) — exhaustive fields

**Identifier:** `tag` — URL-friendly slug, **immutable** (no rename). Validation
regex: single char `^[a-z0-9]$`, else `^[a-z0-9][a-z0-9_-]*[a-z0-9]$`. There is an
internal numeric `id` (in responses) but all addressing is by `tag`.
**→ Terraform import key = `tag`; `tag` change = RequiresReplace.**

### 2a. Top-level monitor columns

| Field | Type | Required | Default | Allowed / notes |
|---|---|---|---|---|
| `tag` | string | yes (create) | — | slug, immutable, unique |
| `name` | string | yes | — | non-empty |
| `description` | string | no | null | |
| `image` | string | no | null | logo/icon URL |
| `cron` | string | no | null | schedule cron expr |
| `default_status` | enum | no | `UP` | `UP\|DOWN\|DEGRADED\|MAINTENANCE\|NO_DATA` |
| `status` | string | no | `ACTIVE` | monitor enabled state: `ACTIVE\|INACTIVE` |
| `category_name` | string | no | null | |
| `monitor_type` | enum | no | `API` | `API\|PING\|TCP\|DNS\|NONE\|GROUP\|SSL\|SQL\|HEARTBEAT\|GAMEDIG\|GRPC` |
| `type_data` | object | no | null | kind-specific (see 2c); PATCH deep-merges |
| `include_degraded_in_downtime` | string | no | `NO` | `YES\|NO` |
| `is_hidden` | string | no | `NO` | `YES\|NO` |
| `confirmation_threshold` | int | no | `1` | **1–60**; not in OpenAPI but validated |
| `monitor_settings_json` | object | no | null | free-form; PATCH deep-merges |
| `external_url` | string\|null | no | null | |

`CreateMonitorRequest` is `additionalProperties:false`. PATCH is partial: omitted
top-level fields keep existing values; `type_data` and `monitor_settings_json`
**merge** rather than replace (send `null` to clear).

### 2b. `type_data` per `monitor_type`

**API (HTTP)**: `url`(req), `method`=GET (`GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS`),
`headers`=`{key,value}[]`, `body`="", `timeout`=10000ms, `allowSelfSignedCert`=false,
`eval`=JS fn (returns `{status,latency}`).

**PING**: `hosts`=`{type,host,timeout,count}[]` (req; per-host `type=IP4`,
`timeout=1000`, `count=3`; `type∈IP4|IP6`), `pingEval`=JS fn.

**TCP**: `hosts`=`{type,host,port,timeout}[]` (req; per-host `type=IP4`, `port=80`,
`timeout=1000`), `tcpEval`=JS fn.

**DNS**: `host`(req), `transport`=UDP (`UDP|TLS`), `nameServer`="" (req for TLS),
`tlsPort`=853, `tlsServername`="", `allowSelfSignedCert`=false, `lookupRecord`=A,
`matchType`=ANY (`ANY|ALL`), `values`=string[] (req non-empty).

**SSL**: `host`(req), `port`="443", `degradedRemainingHours`=168 (> down),
`downRemainingHours`=24.

**SQL**: `dbType`=pg (`pg|mysql2|mssql|oracledb|sqlite3`), `connectionString`(req),
`query`="SELECT 1", `timeout`=5000ms.

**HEARTBEAT** (push): `degradedRemainingMinutes`=5, `downRemainingMinutes`=10 (>
degraded). Push endpoint `GET|POST /ext/heartbeat/{tag}/{secret}`.

**GAMEDIG**: `gameId`(req), `host`(req), `port`=27015, `timeout`=10000 (≥2000),
`guessPort`=false, `requestRules`=false, `eval`=JS fn.

**GRPC**: `host`(req), `port`=50051 (req), `service`="", `tls`=false,
`insecure`=false, `timeout`=10000ms.

**GROUP**: `monitors`=`{tag,weight}[]` (req, ≥2 members, weights sum≈1),
`executionDelay`=1000ms (≥1000), `latencyCalculation`=AVG (`AVG|MAX|MIN`).

**NONE**: placeholder, no `type_data`.

> Design note: because `type_data` is a polymorphic free-form JSON blob (whose exact
> validation lives in per-type JS, incl. `eval` JS functions), the Terraform schema
> should model it as a `jsonencode`-friendly string attribute (or `types.Dynamic`),
> not a rigid nested block. This keeps the provider forward-compatible with new
> monitor types and avoids fighting Kener's server-side deep-merge.

---

## 3. Incidents & Comments

**Incident** (id: numeric). `CreateIncidentRequest` (`additionalProperties:false`):
`title`(req), `start_date_time`(int unix s, req), `end_date_time`(int, opt),
`monitors`=`MonitorImpact[]` (opt). Update: all optional. Response adds `state`,
`status`, `incident_type`, `incident_source`, `url` (absolute — use verbatim).
`MonitorImpact`: `monitor_tag`(req), `impact`(`UP|DOWN|DEGRADED|MAINTENANCE`).

**Incident Comment** (id: numeric, nested). Create (`additionalProperties:false`):
`comment`(req), `state`(req `INVESTIGATING|IDENTIFIED|MONITORING|RESOLVED`),
`timestamp`(int, opt). Update: all optional.

---

## 4. Pages (status pages)

**Identifier:** `page_path` (slug). Home page addressed via **`~home`** (fixed path;
PATCHing `~home` to another path → 400). **→ import key = `page_path` (or `~home`).**

Create (`additionalProperties:false`): `page_path`(req), `page_title`(req),
`page_header`(req), `page_subheader`(opt), `page_logo`(opt), `page_settings`(opt),
`monitors`(opt — **array of monitor tag strings** on write; array of full monitor
objects on read). Update: same optional fields.

`PageSettings` (deep-merged): `incidents`, `include_maintenances`,
`monitor_status_history_days`={desktop:90, mobile:30}, `monitor_layout_style`
(`default-list|default-grid|compact-list|compact-grid`), `meta_page_title`,
`meta_page_description`, `social_page_preview_image`.

---

## 5. Site (global configuration) — update-only keyed singletons

`SiteDataEntry`: `key`(req), `value`(any), `data_type`(`string`|`object`).
`PATCH /api/v4/site/{config_key}` body `{value:<any>}`. No create/delete.

Keys (from `siteDataKeys.ts`): strings — `title, siteName, siteURL, home, favicon,
logo, footerHTML, kenerTheme, customCSS, themeToggle, tzToggle, showSiteStatus,
homeIncidentCount, homeIncidentStartTimeWithin, incidentGroupView,
socialPreviewImage, metaSiteTitle, metaSiteDescription`; enums — `pattern`,
`theme{light,dark,system,none}`, `barStyle{PARTIAL,FULL}`,
`barRoundness{SHARP,ROUNDED}`, `summaryStyle{CURRENT,DAY}`; objects — `homeDataMaxDays,
metaTags, nav, hero, i18n, analytics(+sub-keys), colors, colorsDark, font,
monitorSort, categories, subscriptionsSettings, subMenuOptions, announcement,
dataRetentionPolicy, eventDisplaySettings, globalPageVisibilitySettings,
pageOrderingSettings, dateAndTimeFormat, sitemap,
globalMaintenanceNotificationSettings`.

---

## 6. Maintenances & Events

**Maintenance** (id: numeric). Create (`additionalProperties:false`): `title`(req),
`description`(opt), `start_date_time`(int, req), `rrule`(string RRULE, req),
`duration_seconds`(int ≥1, req), `monitors`=`MonitorImpact[]`(opt). Update: +`status`
(`ACTIVE|INACTIVE`), all optional. Response `url` carries `?type=maintenance`.

**Maintenance Event** (id: numeric, nested, **no create**). Update is `oneOf`:
(1) window edit `{start_date_time, end_date_time}`, or (2) status transition
`{status}` where status ∈ `COMPLETED|CANCELLED` (allowed: `ONGOING→COMPLETED`;
`SCHEDULED|READY|ONGOING→CANCELLED`).

---

## 7. Terraform import-key summary

| Resource | Import key | Type |
|---|---|---|
| monitor | `tag` | slug, immutable (RequiresReplace) |
| incident | `id` | numeric |
| incident comment | `id` (under incident_id) | numeric, nested |
| page | `page_path` (home = `~home`) | slug |
| site config | `config_key` | string, fixed set, update-only |
| maintenance | `id` | numeric |
| maintenance event | `id` (under maintenance_id) | numeric, nested, non-creatable |

---

## 8. API coverage gaps (call-outs)

Kener's `/manage` admin dashboard exposes **triggers/alerts, subscribers, and API
keys/tokens**, but **none of these have a `/api/v4` REST surface** — they are
session-authenticated only (not token), so the provider cannot manage them. The
provider's real resource set is therefore:

- `kener_monitor` (+ data source) — reference resource
- `kener_incident` (+ `kener_incident_comment`)
- `kener_page` (status page)
- `kener_maintenance`
- `kener_site_config` (update-only singleton, per key)

Maintenance **events** and monitor **data points** are read/telemetry-ish and are
better left out (or exposed as data sources only).

## Source URLs
- OpenAPI (authoritative): `https://raw.githubusercontent.com/rajnandan1/kener/main/static/api-references/v4.json`
- Route handlers: `src/routes/(api)/api/v4/**/+server.ts`
- Auth guard: `src/hooks.server.ts`; API-key model: `src/lib/server/controllers/apiController.ts`
- Site keys: `src/lib/server/controllers/siteDataKeys.ts`; monitor enum: `src/lib/types/monitor.ts`
- Monitor type_data docs: `src/routes/(docs)/docs/content/v4/monitors/*.md`
- Docs site: `https://kener.ing/docs`
