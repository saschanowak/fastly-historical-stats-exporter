# Agents Reference: fastly-historical-stats-exporter

This document describes the architecture, conventions, and key patterns for AI agents working on this codebase. Read it before making changes.

---

## What This Project Does

`fastly-historical-stats-exporter` polls the [Fastly Historical Stats API](https://www.fastly.com/documentation/reference/api/metrics-stats/historical-stats/) and exposes the data as Prometheus metrics. It is a pull-based exporter: a background goroutine fetches data from Fastly on a configurable interval and caches the results; when Prometheus scrapes `/metrics`, the collector reads from the cache without any network I/O on the critical path.

This is distinct from the official [fastly/fastly-exporter](https://github.com/fastly/fastly-exporter), which uses the real-time streaming API via long-polling. This exporter uses the REST-based historical stats endpoint (`GET /stats/service/{id}?by=minute`).

---

## Repository Layout

```
.
├── agents.md            ← You are here
├── README.md            ← User-facing documentation
├── Dockerfile           ← Multi-stage scratch build
├── Makefile             ← build, test, docker-build targets
├── go.mod
├── go.sum
├── cmd/
│   └── main.go          ← Entry point: flag parsing, wiring, HTTP server
└── pkg/
    ├── api/
    │   └── client.go    ← Fastly HTTP client
    ├── filter/
    │   └── filter.go    ← Regex allow/block filter
    └── exporter/
        └── collector.go ← prometheus.Collector implementation
```

There is intentionally no internal abstraction for "services" beyond the `api.Service` struct. Keep the codebase small and direct.

---

## Package Responsibilities

### `pkg/filter`

Stateless regex filter. The zero-value `Filter` permits everything (safe default).

- `Allow(expr string) error` — compiles `expr` as a regexp and appends to allowlist
- `Block(expr string) error` — compiles `expr` as a regexp and appends to blocklist
- `Permit(s string) bool` — returns true if `s` passes the filter:
  - If blocklist is non-empty and any blocklist regexp matches `s`: **blocked** (false)
  - If allowlist is non-empty and no allowlist regexp matches `s`: **blocked** (false)
  - Otherwise: **permitted** (true)

The `-metric-allowlist` and `-metric-blocklist` CLI flags compile into this filter. Multiple patterns within the same filter type are OR'd.

### `pkg/api`

Thin HTTP client for two Fastly endpoints:

1. **`ListServices(ctx)`** — `GET /service?per_page=100&page=N`. Paginates by reading the `X-Next-Page` response header. Returns `[]Service{ID, Name, Version}`.

2. **`GetStats(ctx, serviceID, from, to)`** — `GET /stats/service/{id}?by=minute&from=<unix>&to=<unix>`. Returns `[]StatsData`.

`StatsData` is a struct with all Fastly historical stat fields as `float64` JSON-tagged fields (plus `StartTime uint64`). All numeric fields are `float64` to feed directly into Prometheus `GaugeValue`. New fields added by Fastly will be ignored by `encoding/json` until explicitly added to the struct.

Auth is sent as `Fastly-Key: <token>` header on every request.

Non-2xx responses are returned as `*APIError{Code int, Msg string}`. Callers can type-assert to detect 401/403 for startup validation.

### `pkg/exporter`

The `Collector` struct implements `prometheus.Collector`. It has three logical responsibilities:

**1. Descriptor building (`buildDescs`)** — called once in `NewCollector`. Iterates `reflect.TypeOf(api.StatsData{})` fields. For each `float64` field whose `json` tag passes the metric filter, creates a `*prometheus.Desc` with:
  - FQName: `{namespace}_historical_{json_tag}` (e.g. `fastly_historical_requests`)
  - Labels: `service_id`, `service_name`

These descriptors live in `c.descs map[string]*prometheus.Desc` and are reused across every `Collect` call — no allocations per-scrape.

**2. Background goroutine (`run`)** — started by `NewCollector`. Runs two tickers:
  - `scrapeInterval` (default 60s): calls `scrapeAll()` to fetch stats for all cached service IDs
  - `refreshInterval` (default 5m): calls `refreshServices()` to re-discover services, then `scrapeAll()`

The goroutine terminates when the context passed to `NewCollector` is cancelled (i.e. when the process receives SIGINT/SIGTERM).

`scrapeAll` fetches `from = now-120s, to = now-60s` — the last complete 1-minute window. This ensures the Fastly data is finalized before we read it.

**3. Prometheus scrape path (`Describe` + `Collect`)** — `Collect` acquires a read lock, snapshots the cache into a local map, releases the lock, then emits one `prometheus.MustNewConstMetric` per (metric, service) pair. No network I/O occurs during `Collect`.

**Why `MustNewConstMetric` instead of `GaugeVec`?**
`GaugeVec` accumulates label combinations and has a stale-label problem: if a service is removed, its labels persist in the vec until `Reset()` is called. Using pre-built `*prometheus.Desc` + `MustNewConstMetric` in `Collect` is the idiomatic pattern for dynamic label sets — each scrape emits exactly the services currently in cache, no more.

### `cmd/main.go`

Entry point. Responsibility: flag parsing, token resolution, filter construction, wiring all packages together, starting the HTTP server, and handling graceful shutdown via `signal.NotifyContext`.

The `programVersion` variable is set at build time via `-ldflags="-X main.programVersion=..."`.

---

## Data Flow

```
Fastly API
    │
    │  (every scrapeInterval)
    ▼
api.Client.GetStats()
    │
    ▼
exporter.Collector.cache   ←  protected by sync.RWMutex
    │
    │  (on every Prometheus scrape)
    ▼
Collector.Collect()
    │  reflect over StatsData fields
    │  filter via c.descs lookup
    ▼
prometheus.MustNewConstMetric → chan<- prometheus.Metric
    │
    ▼
/metrics HTTP endpoint
```

---

## Metric Naming Convention

```
{namespace}_historical_{field_name}
```

- Default namespace: `fastly`
- Example: `fastly_historical_requests{service_id="abc",service_name="My Service"}`
- Namespace is configurable via `-namespace` flag
- All metrics are **Gauge** type (window-scoped counts, not cumulative)

Self-monitoring metrics use a fixed prefix: `fastly_historical_exporter_{name}` (not subject to the metric filter).

---

## Adding a New Fastly Metric Field

1. Add a `float64` field to `api.StatsData` with the correct `json:"..."` tag matching the Fastly API field name.
2. That is all. The reflection loop in `buildDescs()` and `Collect()` will pick it up automatically.

## Adding a New CLI Flag

1. Declare the variable in `cmd/main.go`.
2. Register it with `flag.StringVar` / `flag.DurationVar` / `flag.Var` before `flag.Parse()`.
3. Pass it to the relevant constructor (`NewCollector`, `NewClient`, etc.).
4. Document it in `README.md`.

---

## Thread Safety

| State | Writer | Readers | Protection |
|-------|--------|---------|------------|
| `c.cache` | `run()` goroutine | `Collect()` | `sync.RWMutex` |
| `c.descs` | `buildDescs()` (once, before goroutine) | `Collect()`, `Describe()` | None needed (read-only after init) |
| Self-monitoring counters/gauges | `run()` goroutine | Prometheus registry | prometheus library internal |

---

## Testing Approach

- `pkg/filter`: pure unit tests, no external deps
- `pkg/api`: uses `net/http/httptest.NewServer` to serve canned JSON; no real Fastly token needed
- `pkg/exporter`: integration-style tests using `httptest.NewServer` for the API; calls `refreshServices` + `scrapeAll` directly (bypasses tickers); uses `prometheus.NewPedanticRegistry().Gather()` to inspect emitted metrics
- No mocks for `api.Client` — tests use the real client pointed at a test server

Run all tests: `go test ./...`
Run with race detector: `go test -race ./...`

---

## Docker Image

The Dockerfile uses a two-stage build:
- Builder: `golang:1.23` — compiles a static binary (`CGO_ENABLED=0`)
- Final: `scratch` — only the binary + CA certificates + `/etc/passwd`

The binary runs as a non-root user (`exporter`) and listens on `0.0.0.0:8080` by default inside the container.

---

## What This Project Is NOT

- Not a replacement for the official `fastly/fastly-exporter` (which exports real-time datacenter-level metrics with per-datacenter labels)
- Not a general Fastly API client library
- Not a multi-tenant or sharded exporter (no service sharding like the official exporter)
- Not a service-mesh or infrastructure-management tool

Keep the scope narrow and the implementation direct.
