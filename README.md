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

**Traffic:** `requests`, `hits`, `miss`, `pass`, `errors`, `synth`, `hit_ratio`, `edge_requests`, `origin_offload`, `pipe`, `restarts`, `uncacheable`, `upgrade`, `request_collapse_usable_count`, `request_collapse_unusable_count`, `request_denied_get_head_body`

**Bandwidth:** `bandwidth`, `resp_body_bytes`, `resp_header_bytes`, `bereq_body_bytes`, `bereq_header_bytes`, `edge_resp_body_bytes`, `edge_resp_header_bytes`, `req_body_bytes`, `req_header_bytes`, `body_size`, `header_size`

**Response bytes by type:** `hit_resp_body_bytes`, `miss_resp_body_bytes`, `pass_resp_body_bytes`

**Timing:** `hits_time`, `miss_time`, `pass_time`

**Status codes:** `status_1xx`, `status_2xx`, `status_3xx`, `status_4xx`, `status_5xx`, `status_200`, `status_204`, `status_206`, `status_301`, `status_302`, `status_304`, `status_400`, `status_401`, `status_403`, `status_404`, `status_406`, `status_410`, `status_416`, `status_422`, `status_429`, `status_500`, `status_501`, `status_502`, `status_503`, `status_504`, `status_505`, `status_530`

**TLS/Protocol:** `tls`, `tls_v10`, `tls_v11`, `tls_v12`, `tls_v13`, `tls_handshake_sent_bytes`, `http2`, `http3`, `ipv6`

**Image Optimizer:** `imgopto`, `imgopto_transforms`, `imgopto_resp_body_bytes`, `imgopto_resp_header_bytes`, `imgopto_shield`, `imgopto_shield_resp_body_bytes`, `imgopto_shield_resp_header_bytes`, `imgopto_compute_requests`, `imgopto_avif_count`, `imgopto_gif_count`, `imgopto_jpeg_count`, `imgopto_jpegxl_count`, `imgopto_mp4_count`, `imgopto_png_count`, `imgopto_svg_count`, `imgopto_webp_count`

**Image Video:** `imgvideo`, `imgvideo_frames`, `imgvideo_resp_body_bytes`, `imgvideo_resp_header_bytes`, `imgvideo_shield`, `imgvideo_shield_frames`, `imgvideo_shield_resp_body_bytes`, `imgvideo_shield_resp_header_bytes`

**Compute:** `compute_requests`, `compute_execution_time_ms`, `compute_ram_used`, `compute_request_time_ms`, `compute_request_time_billed_ms`, `compute_bereqs`, `compute_bereq_body_bytes`, `compute_bereq_header_bytes`, `compute_bereq_errors`, `compute_beresp_body_bytes`, `compute_beresp_header_bytes`, `compute_req_body_bytes`, `compute_req_header_bytes`, `compute_resp_body_bytes`, `compute_resp_header_bytes`, `compute_resp_status_103`, `compute_resp_status_1xx`, `compute_resp_status_200`, `compute_resp_status_204`, `compute_resp_status_206`, `compute_resp_status_2xx`, `compute_resp_status_301`, `compute_resp_status_302`, `compute_resp_status_304`, `compute_resp_status_3xx`, `compute_resp_status_400`, `compute_resp_status_401`, `compute_resp_status_403`, `compute_resp_status_404`, `compute_resp_status_416`, `compute_resp_status_429`, `compute_resp_status_4xx`, `compute_resp_status_500`, `compute_resp_status_501`, `compute_resp_status_502`, `compute_resp_status_503`, `compute_resp_status_504`, `compute_resp_status_505`, `compute_resp_status_530`, `compute_resp_status_5xx`, `compute_cache_operations_count`, `compute_sandboxes`, `compute_handoff`, `compute_guest_errors`, `compute_runtime_errors`, `compute_globals_limit_exceeded`, `compute_heap_limit_exceeded`, `compute_stack_limit_exceeded`, `compute_resource_limit_exceeded`, `compute_platform_internal_error`, `compute_platform_invalid_request_error`, `compute_service_bereq_error`, `compute_service_chain_error`, `compute_service_limits_error`, `compute_service_memory_exceeded_error`, `compute_service_resource_limits_error`, `compute_service_runtime_error`, `compute_service_timeout_error`, `compute_service_vcpu_exceeded_error`

**Next-Gen WAF (NGWAF):** `ngwaf_requests_total_count`, `ngwaf_requests_allowed_count`, `ngwaf_requests_blocked_count`, `ngwaf_requests_challenged_count`, `ngwaf_requests_logged_count`, `ngwaf_requests_timeout_count`, `ngwaf_requests_unknown_count`, `ngwaf_bot_analysis_request_count`

**WAF (legacy):** `waf_blocked`, `waf_logged`, `waf_passed`

**DDoS:** `ddos_action_blackhole`, `ddos_action_close`, `ddos_action_tarpit`, `ddos_action_tarpit_accept`, `ddos_action_downgrade`, `ddos_action_downgraded_connections`, `ddos_action_limit_streams_connections`, `ddos_action_limit_streams_requests`, `ddos_protection_requests_allow_count`, `ddos_protection_requests_detect_count`, `ddos_protection_requests_mitigate_count`

**Shield:** `shield`, `shield_resp_body_bytes`, `shield_resp_header_bytes`, `shield_fetches`, `shield_fetch_body_bytes`, `shield_fetch_header_bytes`, `shield_fetch_resp_body_bytes`, `shield_fetch_resp_header_bytes`, `shield_cache_fetches`, `shield_hit_requests`, `shield_hit_resp_body_bytes`, `shield_hit_resp_header_bytes`, `shield_miss_requests`, `shield_miss_resp_body_bytes`, `shield_miss_resp_header_bytes`, `shield_revalidations`

**Bot management:** `bot_challenges_issued`, `bot_challenges_succeeded`, `bot_challenges_failed`, `bot_challenge_starts`, `bot_challenge_complete_tokens_checked`, `bot_challenge_complete_tokens_disabled`, `bot_challenge_complete_tokens_failed`, `bot_challenge_complete_tokens_issued`, `bot_challenge_complete_tokens_passed`, `bot_requests_total_count`, `bot_edge_requests_analyzed_count`, `bot_edge_requests_detected_count`, `bot_edge_requests_verified_count`, `bot_edge_requests_accessibility_count`, `bot_edge_requests_ai_crawler_count`, `bot_edge_requests_ai_fetcher_count`, `bot_edge_requests_content_fetcher_count`, `bot_edge_requests_monitoring_count`, `bot_edge_requests_online_marketing_count`, `bot_edge_requests_page_preview_count`, `bot_edge_requests_platform_integrations_count`, `bot_edge_requests_research_count`, `bot_edge_requests_search_engine_crawler_count`, `bot_edge_requests_search_engine_optimization_count`, `bot_edge_requests_security_tools_count`

**Video:** `otfp`, `otfp_deliver_time`, `otfp_resp_body_bytes`, `otfp_resp_header_bytes`, `otfp_manifest_resp_body_bytes`, `otfp_manifest_resp_header_bytes`, `otfp_manifests`, `otfp_shield_resp_body_bytes`, `otfp_shield_resp_header_bytes`, `otfp_shield_time`, `video`, `pci`

**Attack:** `attack_req_body_bytes`, `attack_req_header_bytes`, `attack_resp_synth_bytes`, `attack_blocked_req_body_bytes`, `attack_blocked_req_header_bytes`, `attack_logged_req_body_bytes`, `attack_logged_req_header_bytes`, `attack_passed_req_body_bytes`, `attack_passed_req_header_bytes`

**Logging:** `log`, `log_bytes`

**VCL subroutines:** `recv_sub_time`, `recv_sub_count`, `hash_sub_time`, `hash_sub_count`, `miss_sub_time`, `miss_sub_count`, `fetch_sub_time`, `fetch_sub_count`, `pass_sub_time`, `pass_sub_count`, `pipe_sub_time`, `pipe_sub_count`, `deliver_sub_time`, `deliver_sub_count`, `error_sub_time`, `error_sub_count`, `hit_sub_time`, `hit_sub_count`, `prehash_sub_time`, `prehash_sub_count`, `predeliver_sub_time`, `predeliver_sub_count`

**Edge breakdown:** `edge_hit_requests`, `edge_hit_resp_body_bytes`, `edge_hit_resp_header_bytes`, `edge_miss_requests`, `edge_miss_resp_body_bytes`, `edge_miss_resp_header_bytes`

**All-sources aggregates:** `all_hit_requests`, `all_miss_requests`, `all_pass_requests`, `all_error_requests`, `all_synth_requests`, `all_edge_hit_requests`, `all_edge_miss_requests`, `all_status_1xx`, `all_status_2xx`, `all_status_3xx`, `all_status_4xx`, `all_status_5xx`

**Origin:** `origin_fetches`, `origin_fetch_body_bytes`, `origin_fetch_header_bytes`, `origin_fetch_resp_body_bytes`, `origin_fetch_resp_header_bytes`, `origin_cache_fetches`, `origin_cache_fetch_resp_body_bytes`, `origin_cache_fetch_resp_header_bytes`, `origin_revalidations`

**WebSocket:** `websocket_req_body_bytes`, `websocket_req_header_bytes`, `websocket_resp_body_bytes`, `websocket_resp_header_bytes`, `websocket_bereq_body_bytes`, `websocket_bereq_header_bytes`, `websocket_beresp_body_bytes`, `websocket_beresp_header_bytes`, `websocket_conn_time_ms`

**Fanout:** `fanout_req_body_bytes`, `fanout_req_header_bytes`, `fanout_resp_body_bytes`, `fanout_resp_header_bytes`, `fanout_bereq_body_bytes`, `fanout_bereq_header_bytes`, `fanout_beresp_body_bytes`, `fanout_beresp_header_bytes`, `fanout_conn_time_ms`, `fanout_recv_publishes`, `fanout_send_publishes`

**KV Store:** `kv_store_class_a_operations`, `kv_store_class_b_operations`

**Object Storage:** `object_store_class_a_operations`, `object_store_class_b_operations`, `object_storage_class_a_operations_count`, `object_storage_class_b_operations_count`

**Object size buckets:** `object_size_1k`, `object_size_10k`, `object_size_100k`, `object_size_1m`, `object_size_10m`, `object_size_100m`, `object_size_1g`

**Segblock:** `segblock_origin_fetches`, `segblock_shield_fetches`

**DNS:** `dns_billable_responses_count`, `dns_nonbillable_responses_count`

**API Discovery:** `api_discovery_requests_count`

**AI Accelerator:** `aia_requests`, `aia_estimated_time_saved_ms`, `aia_origin_usage_tokens`, `aia_response_usage_tokens`, `aia_status_1xx`, `aia_status_2xx`, `aia_status_3xx`, `aia_status_4xx`, `aia_status_5xx`

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
# fastly-historical-stats-exporter
