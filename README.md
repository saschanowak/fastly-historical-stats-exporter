# fastly-historical-stats-exporter

A [Prometheus](https://prometheus.io) exporter for the [Fastly Historical Stats API](https://www.fastly.com/documentation/reference/api/metrics-stats/historical-stats/).

It exports per-service metrics (requests, bandwidth, cache hit ratio, status codes, TLS versions, and more) by polling the Fastly REST API on a configurable interval.

> **Note:** This exporter uses the *historical* (REST) API, not the real-time streaming API. Metrics reflect the last completed 1-minute window as reported by Fastly. For per-datacenter, real-time metrics use the official [fastly-exporter](https://github.com/fastly/fastly-exporter) instead.

---

## Installation

### Binary

Download a pre-built binary from the [releases page](https://github.com/saschanowak/fastly-historical-stats-exporter/releases).

```sh
./fastly-historical-stats-exporter -token YOUR_API_TOKEN
```

### Docker

```sh
docker run \
  -e FASTLY_API_TOKEN=YOUR_API_TOKEN \
  -p 8080:8080 \
  ghcr.io/saschanowak/fastly-historical-stats-exporter:latest
```

### Source

Requires Go 1.23 or newer.

```sh
git clone https://github.com/saschanowak/fastly-historical-stats-exporter
cd fastly-historical-stats-exporter
make build
./fastly-historical-stats-exporter -token YOUR_API_TOKEN
```

---

## Using the Exporter

### Authentication

A valid Fastly API token is required. The token must have at least **read access** to the services you want to export.

Provide the token via the `-token` flag:

```sh
./fastly-historical-stats-exporter -token YOUR_API_TOKEN
```

Or via the `FASTLY_API_TOKEN` environment variable (the flag takes precedence if both are set):

```sh
export FASTLY_API_TOKEN=YOUR_API_TOKEN
./fastly-historical-stats-exporter
```

### Basic Usage

By default, the exporter discovers and exports all services accessible to your token, listening on `:8080`.

```sh
./fastly-historical-stats-exporter -token YOUR_API_TOKEN
```

Visit `http://localhost:8080/metrics` to see exported metrics.

### Filtering Services

By default, all services available to your API token are exported. To export only specific services, use the `-service` flag (repeatable):

```sh
./fastly-historical-stats-exporter \
  -token YOUR_API_TOKEN \
  -service SVC_ID_1 \
  -service SVC_ID_2
```

Find your service IDs in the Fastly web console or via the API.

### Filtering Metrics

By default, all metrics provided by the Fastly Historical Stats API are exported. You can limit the exported set in two ways:

**Allowlist** — export only metrics whose name matches a regex:

```sh
./fastly-historical-stats-exporter \
  -token YOUR_API_TOKEN \
  -metric-allowlist 'bytes$'
```

**Blocklist** — exclude metrics whose name matches a regex:

```sh
./fastly-historical-stats-exporter \
  -token YOUR_API_TOKEN \
  -metric-blocklist 'imgopto'
```

Both flags accept a single [RE2](https://github.com/google/re2/wiki/Syntax) regular expression. The allowlist and blocklist can be combined: a metric is exported only if it matches the allowlist (if set) and does not match the blocklist (if set).

### Filter Semantics

| allowlist | blocklist | Result |
|-----------|-----------|--------|
| not set | not set | all metrics exported |
| set | not set | only matching metrics exported |
| not set | set | all except matching metrics exported |
| set | set | metrics matching allowlist AND not matching blocklist |

### Polling Interval

Stats are fetched from the Fastly API every 60 seconds by default. The exporter always fetches the last completed 1-minute window (i.e. `from=now-120s, to=now-60s`).

```sh
./fastly-historical-stats-exporter \
  -token YOUR_API_TOKEN \
  -scrape-interval 30s
```

### Service List Refresh

The exporter refreshes the list of available services every 5 minutes by default. To change the interval:

```sh
./fastly-historical-stats-exporter \
  -token YOUR_API_TOKEN \
  -refresh-interval 10m
```

---

## All Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-token` | | Fastly API token (`FASTLY_API_TOKEN` env var if not set) |
| `-service` | all services | Explicit service ID to export (repeatable) |
| `-listen` | `:8080` | TCP address to listen on |
| `-metric-allowlist` | | Regex; export only matching metric names |
| `-metric-blocklist` | | Regex; exclude matching metric names |
| `-namespace` | `fastly` | Prometheus metric namespace prefix |
| `-scrape-interval` | `60s` | How often to fetch stats from the Fastly API |
| `-refresh-interval` | `5m` | How often to refresh the list of Fastly services |

---

## Metrics

All Fastly stats are exported as Prometheus **Gauges** with the naming pattern:

```
fastly_historical_{field_name}{service_id="...", service_name="..."}
```

The metric values reflect counts and bytes for the last complete 1-minute window as reported by the Fastly API.

### Example Metrics

```
# Traffic
fastly_historical_requests{service_id="abc",service_name="My CDN"} 12345
fastly_historical_hits{service_id="abc",service_name="My CDN"} 10000
fastly_historical_miss{service_id="abc",service_name="My CDN"} 2000
fastly_historical_pass{service_id="abc",service_name="My CDN"} 300
fastly_historical_errors{service_id="abc",service_name="My CDN"} 12
fastly_historical_hit_ratio{service_id="abc",service_name="My CDN"} 0.812

# Bandwidth (bytes)
fastly_historical_bandwidth{service_id="abc",service_name="My CDN"} 5.12e+08
fastly_historical_resp_body_bytes{service_id="abc",service_name="My CDN"} 4.8e+08
fastly_historical_resp_header_bytes{service_id="abc",service_name="My CDN"} 3.2e+07

# HTTP status codes
fastly_historical_status_2xx{service_id="abc",service_name="My CDN"} 10000
fastly_historical_status_4xx{service_id="abc",service_name="My CDN"} 200
fastly_historical_status_5xx{service_id="abc",service_name="My CDN"} 12

# TLS versions
fastly_historical_tls_v12{service_id="abc",service_name="My CDN"} 6000
fastly_historical_tls_v13{service_id="abc",service_name="My CDN"} 4000

# HTTP versions
fastly_historical_http2{service_id="abc",service_name="My CDN"} 8000
fastly_historical_http3{service_id="abc",service_name="My CDN"} 500
```

### Exporter Self-Monitoring Metrics

```
fastly_historical_exporter_scrape_duration_seconds     # duration of last fetch cycle
fastly_historical_exporter_scrape_errors_total         # total API errors
fastly_historical_exporter_services_discovered         # services currently tracked
fastly_historical_exporter_last_scrape_timestamp_seconds  # last successful scrape
```

### Complete Field List

The following fields from the Fastly Historical Stats API are exported. See the [Fastly API documentation](https://www.fastly.com/documentation/reference/api/metrics-stats/historical-stats/) for descriptions of each field.

**Traffic:** `requests`, `hits`, `miss`, `pass`, `errors`, `synth`, `hit_ratio`, `edge_requests`, `origin_offload`

**Bandwidth:** `bandwidth`, `resp_body_bytes`, `resp_header_bytes`, `bereq_body_bytes`, `bereq_header_bytes`, `edge_resp_body_bytes`, `edge_resp_header_bytes`

**Status codes:** `status_1xx`, `status_2xx`, `status_3xx`, `status_4xx`, `status_5xx`, `status_200`, `status_301`, `status_302`, `status_304`, `status_400`, `status_401`, `status_403`, `status_404`, `status_410`, `status_416`, `status_422`, `status_503`

**Timing:** `hits_time`, `miss_time`

**TLS/Protocol:** `tls`, `tls_v10`, `tls_v11`, `tls_v12`, `tls_v13`, `http2`, `http3`, `ipv6`

**Image Optimizer:** `imgopto`, `imgopto_transforms`, `imgopto_resp_body_bytes`, `imgopto_resp_header_bytes`

**Compute:** `compute_requests`, `compute_execution_time_ms`, `compute_ram_used`

**WAF:** `waf_blocked`, `waf_logged`, `waf_passed`

**DDoS:** `ddos_action_blackhole`, `ddos_action_close`, `ddos_action_tarpit`

**Shield:** `shield`, `shield_resp_body_bytes`, `shield_resp_header_bytes`

**Bot management:** `bot_challenges_issued`, `bot_challenges_succeeded`, `bot_challenges_failed`

**Video:** `otfp`, `otfp_deliver_time`, `otfp_resp_body_bytes`, `otfp_resp_header_bytes`, `otfp_manifest_resp_body_bytes`, `otfp_manifest_resp_header_bytes`, `video`, `pci`

**Attack:** `attack_req_body_bytes`, `attack_req_header_bytes`, `attack_resp_synth_bytes`

**Logging:** `log`, `log_bytes`

**VCL subroutines:** `recv_sub_time`, `recv_sub_count`, `hash_sub_time`, `hash_sub_count`, `miss_sub_time`, `miss_sub_count`, `fetch_sub_time`, `fetch_sub_count`, `pass_sub_time`, `pass_sub_count`, `pipe_sub_time`, `pipe_sub_count`, `deliver_sub_time`, `deliver_sub_count`, `error_sub_time`, `error_sub_count`, `hit_sub_time`, `hit_sub_count`, `prehash_sub_time`, `prehash_sub_count`, `predeliver_sub_time`, `predeliver_sub_count`

---

## Prometheus Configuration

Add a scrape job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: fastly_historical
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 60s
    scrape_timeout: 30s
```

> Set `scrape_interval` to match the exporter's `-scrape-interval` (default 60s) so Prometheus always sees fresh data.

---

## Example Queries

Cache hit ratio as a percentage:

```promql
fastly_historical_hit_ratio * 100
```

Total bandwidth per service over the last 5 minutes:

```promql
sum by (service_name) (fastly_historical_bandwidth)
```

Error rate (5xx / total requests):

```promql
fastly_historical_status_5xx / fastly_historical_requests
```

TLS 1.3 adoption:

```promql
fastly_historical_tls_v13 / fastly_historical_tls
```

---

## Building

```sh
make build      # compile binary
make test       # run tests with race detector
make lint       # go vet
make docker-build  # build Docker image
```

---

## License

Apache 2.0. See [LICENSE](LICENSE).
