package exporter_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/api"
	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/exporter"
	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/filter"
)

// discardLogger returns a slog.Logger that throws away all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

// newFastlyServer returns an httptest.Server that serves a minimal but valid
// Fastly API for /service and /stats/service/{id}.
func newFastlyServer(services []api.Service, statsData []map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/service":
			json.NewEncoder(w).Encode(services)
		default:
			// /stats/service/{id}
			json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   statsData,
			})
		}
	}))
}

// collectAll drains the Collector into a slice of Metrics.
func collectAll(t *testing.T, c *exporter.Collector) []prometheus.Metric {
	t.Helper()
	ch := make(chan prometheus.Metric, 256)
	c.Collect(ch)
	close(ch)
	var out []prometheus.Metric
	for m := range ch {
		out = append(out, m)
	}
	return out
}

// describeAll drains Describe into a slice of Descs.
func describeAll(t *testing.T, c *exporter.Collector) []*prometheus.Desc {
	t.Helper()
	ch := make(chan *prometheus.Desc, 512)
	c.Describe(ch)
	close(ch)
	var out []*prometheus.Desc
	for d := range ch {
		out = append(out, d)
	}
	return out
}

// ---------- Describe ----------

func TestDescribeEmitsDescriptors(t *testing.T) {
	srv := newFastlyServer(nil, nil)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)

	descs := describeAll(t, c)
	if len(descs) == 0 {
		t.Fatal("Describe emitted no descriptors")
	}
}

func TestDescribeIncludesSelfMetrics(t *testing.T) {
	srv := newFastlyServer(nil, nil)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)

	descs := describeAll(t, c)

	// Count descriptors whose fqName contains "exporter" (self-metrics).
	selfCount := 0
	for _, d := range descs {
		if containsStr(d.String(), "historical_exporter") {
			selfCount++
		}
	}
	// We expect exactly 4 self-monitoring metrics.
	if selfCount != 4 {
		t.Errorf("expected 4 self-monitoring descriptors, got %d", selfCount)
	}
}

func TestDescribeRespectsAllowlistFilter(t *testing.T) {
	srv := newFastlyServer(nil, nil)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := &filter.Filter{}
	if err := f.Allow("^requests$"); err != nil {
		t.Fatal(err)
	}

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		f,
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)

	descs := describeAll(t, c)

	// Only "requests" + the 4 self-metrics should be present.
	statsDescs := 0
	for _, d := range descs {
		if !containsStr(d.String(), "historical_exporter") {
			statsDescs++
		}
	}
	if statsDescs != 1 {
		t.Errorf("expected 1 stats descriptor after allow-filter, got %d", statsDescs)
	}
}

func TestDescribeRespectsBlocklistFilter(t *testing.T) {
	srv := newFastlyServer(nil, nil)
	defer srv.Close()

	// Block a whole category; verify those descriptors are absent.
	f := &filter.Filter{}
	if err := f.Block("^imgopto"); err != nil {
		t.Fatal(err)
	}

	noFilter := &filter.Filter{}

	ctxFull, cancelFull := context.WithCancel(context.Background())
	defer cancelFull()

	full := exporter.NewCollector(
		ctxFull,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		noFilter,
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)
	filtered := cFor(t, srv.URL, f)
	defer filtered.Cancel()

	fullCount := statsDescCount(t, full)
	filteredCount := statsDescCount(t, filtered.Collector)
	if filteredCount >= fullCount {
		t.Errorf("blocked filter should reduce descriptor count: full=%d filtered=%d", fullCount, filteredCount)
	}
}

// ---------- Collect ----------

func TestCollectEmitsMetricsForKnownService(t *testing.T) {
	services := []api.Service{{ID: "svc1", Name: "Test Service"}}
	stats := []map[string]any{
		{"start_time": 1700000000, "requests": 42.0, "hits": 30.0},
	}

	srv := newFastlyServer(services, stats)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)

	metrics := collectAll(t, c)
	if len(metrics) == 0 {
		t.Fatal("Collect emitted no metrics")
	}

	// At least one metric must mention the service ID in its label values.
	found := false
	for _, m := range metrics {
		if containsStr(m.Desc().String(), "service_id") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one metric with service_id label")
	}
}

func TestCollectEmitsNoStatsMetricsWithNoServices(t *testing.T) {
	srv := newFastlyServer(nil, nil) // no services
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)

	metrics := collectAll(t, c)

	// Only self-monitoring metrics (4) should appear; no per-service stats.
	for _, m := range metrics {
		if containsStr(m.Desc().String(), "service_id") {
			t.Error("expected no service-level metrics when there are no services")
			break
		}
	}
}

func TestCollectWithNamespacePrefix(t *testing.T) {
	services := []api.Service{{ID: "svc1", Name: "S"}}
	stats := []map[string]any{{"start_time": 1700000000, "requests": 1.0}}

	srv := newFastlyServer(services, stats)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"myns",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)

	descs := describeAll(t, c)
	foundNs := false
	for _, d := range descs {
		if containsStr(d.String(), "myns_historical") {
			foundNs = true
			break
		}
	}
	if !foundNs {
		t.Error("expected descriptor with namespace prefix 'myns_historical'")
	}
}

// ---------- Explicit service IDs ----------

func TestExplicitServiceIDsSkipsDiscovery(t *testing.T) {
	discoveryCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/service" {
			discoveryCalled = true
			json.NewEncoder(w).Encode([]api.Service{})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []any{}})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"fastly",
		time.Hour, time.Hour,
		[]string{"explicit-svc-1"},
		discardLogger(),
	)

	if discoveryCalled {
		t.Error("GET /service was called even though explicit service IDs were provided")
	}
}

func TestExplicitServiceIDsAppearInMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/service" {
			json.NewEncoder(w).Encode([]api.Service{})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   []map[string]any{{"start_time": 1700000000, "requests": 5.0}},
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(srv.URL)),
		&filter.Filter{},
		"fastly",
		time.Hour, time.Hour,
		[]string{"my-explicit-id"},
		discardLogger(),
	)

	metrics := collectAll(t, c)
	found := false
	for _, m := range metrics {
		var dto prometheus.Metric
		_ = dto
		// Check via the metric's string representation.
		if containsStr(m.Desc().String(), "service_id") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected metrics for explicit service ID, got none")
	}
}

// ---------- helpers ----------

type collectorWithCancel struct {
	*exporter.Collector
	Cancel context.CancelFunc
}

func cFor(t *testing.T, baseURL string, f *filter.Filter) collectorWithCancel {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := exporter.NewCollector(
		ctx,
		api.NewClient("tok", api.WithBaseURL(baseURL)),
		f,
		"fastly",
		time.Hour, time.Hour, nil,
		discardLogger(),
	)
	return collectorWithCancel{Collector: c, Cancel: cancel}
}

func statsDescCount(t *testing.T, c *exporter.Collector) int {
	t.Helper()
	descs := describeAll(t, c)
	n := 0
	for _, d := range descs {
		if !containsStr(d.String(), "historical_exporter") {
			n++
		}
	}
	return n
}

func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
