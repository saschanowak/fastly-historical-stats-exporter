// Package exporter implements a Prometheus collector for Fastly historical stats.
package exporter

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/api"
	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/filter"
)

type serviceCache struct {
	service api.Service
	data    []api.StatsData
}

// Collector implements prometheus.Collector. A background goroutine fetches
// Fastly stats on scrapeInterval and caches them; Collect reads from the
// cache with no network I/O on the scrape path.
type Collector struct {
	client          *api.Client
	filter          *filter.Filter
	namespace       string
	scrapeInterval  time.Duration
	refreshInterval time.Duration
	explicitIDs     []string

	mu    sync.RWMutex
	cache map[string]serviceCache

	// pre-built descriptors keyed by StatsData json tag; built once at init
	descs map[string]*prometheus.Desc

	// self-monitoring metrics (not subject to metric filter)
	scrapeDuration prometheus.Gauge
	scrapeErrors   prometheus.Counter
	servicesCount  prometheus.Gauge
	lastScrapeTime prometheus.Gauge
}

// NewCollector creates a Collector, builds Prometheus descriptors, performs an
// initial service refresh + stats scrape, and starts the background goroutine.
func NewCollector(
	ctx context.Context,
	client *api.Client,
	metricFilter *filter.Filter,
	namespace string,
	scrapeInterval time.Duration,
	refreshInterval time.Duration,
	explicitIDs []string,
	logger *slog.Logger,
) *Collector {
	c := &Collector{
		client:          client,
		filter:          metricFilter,
		namespace:       namespace,
		scrapeInterval:  scrapeInterval,
		refreshInterval: refreshInterval,
		explicitIDs:     explicitIDs,
		cache:           make(map[string]serviceCache),
	}

	c.descs = c.buildDescs()
	c.initSelfMetrics()

	// Initial population so the first Prometheus scrape has data.
	if err := c.refreshServices(ctx); err != nil {
		logger.Warn("initial service refresh failed", "err", err)
	}
	if err := c.scrapeAll(ctx); err != nil {
		logger.Warn("initial scrape failed", "err", err)
	}

	go c.run(ctx, logger)
	return c
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descs {
		ch <- d
	}
	c.scrapeDuration.Describe(ch)
	c.scrapeErrors.Describe(ch)
	c.servicesCount.Describe(ch)
	c.lastScrapeTime.Describe(ch)
}

// Collect implements prometheus.Collector.
// Reads from the in-memory cache and emits one metric per (field, service) pair.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	snapshot := make(map[string]serviceCache, len(c.cache))
	for k, v := range c.cache {
		snapshot[k] = v
	}
	c.mu.RUnlock()

	statsType := reflect.TypeOf(api.StatsData{})

	for _, sc := range snapshot {
		if len(sc.data) == 0 {
			continue
		}
		d := sc.data[len(sc.data)-1] // most recent complete bucket
		statsVal := reflect.ValueOf(d)

		for i := 0; i < statsType.NumField(); i++ {
			field := statsType.Field(i)
			if field.Type.Kind() != reflect.Float64 {
				continue
			}
			tag := field.Tag.Get("json")
			desc, ok := c.descs[tag]
			if !ok {
				continue // filtered out
			}
			ch <- prometheus.MustNewConstMetric(
				desc,
				prometheus.GaugeValue,
				statsVal.Field(i).Float(),
				sc.service.ID, sc.service.Name,
			)
		}
	}

	c.scrapeDuration.Collect(ch)
	c.scrapeErrors.Collect(ch)
	c.servicesCount.Collect(ch)
	c.lastScrapeTime.Collect(ch)
}

// buildDescs iterates StatsData float64 fields and creates a Prometheus
// descriptor for each field that passes the metric filter.
func (c *Collector) buildDescs() map[string]*prometheus.Desc {
	descs := make(map[string]*prometheus.Desc)
	t := reflect.TypeOf(api.StatsData{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() != reflect.Float64 {
			continue
		}
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			continue
		}
		if !c.filter.Permit(name) {
			continue
		}
		descs[name] = prometheus.NewDesc(
			prometheus.BuildFQName(c.namespace, "historical", name),
			"Fastly historical stat: "+name,
			[]string{"service_id", "service_name"},
			nil,
		)
	}
	return descs
}

func (c *Collector) initSelfMetrics() {
	c.scrapeDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: c.namespace,
		Subsystem: "historical_exporter",
		Name:      "scrape_duration_seconds",
		Help:      "Duration in seconds of the last Fastly stats fetch cycle.",
	})
	c.scrapeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Subsystem: "historical_exporter",
		Name:      "scrape_errors_total",
		Help:      "Total number of errors fetching stats from the Fastly API.",
	})
	c.servicesCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: c.namespace,
		Subsystem: "historical_exporter",
		Name:      "services_discovered",
		Help:      "Number of Fastly services currently tracked by the exporter.",
	})
	c.lastScrapeTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: c.namespace,
		Subsystem: "historical_exporter",
		Name:      "last_scrape_timestamp_seconds",
		Help:      "Unix timestamp of the last successful Fastly stats fetch.",
	})
}

// run is the background goroutine. It drives two tickers for stats fetching
// and service list refresh. It exits when ctx is cancelled.
func (c *Collector) run(ctx context.Context, logger *slog.Logger) {
	scrapeTick := time.NewTicker(c.scrapeInterval)
	refreshTick := time.NewTicker(c.refreshInterval)
	defer scrapeTick.Stop()
	defer refreshTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-refreshTick.C:
			if err := c.refreshServices(ctx); err != nil {
				logger.Warn("service refresh failed", "err", err)
			}
			if err := c.scrapeAll(ctx); err != nil {
				logger.Warn("stats scrape failed after refresh", "err", err)
			}
		case <-scrapeTick.C:
			if err := c.scrapeAll(ctx); err != nil {
				logger.Warn("stats scrape failed", "err", err)
			}
		}
	}
}

// refreshServices re-discovers services from the Fastly API and merges them
// with any explicitly specified service IDs.
func (c *Collector) refreshServices(ctx context.Context) error {
	var services []api.Service

	if len(c.explicitIDs) > 0 {
		// Use explicitly provided IDs; no API discovery needed.
		// We still need names, so we build placeholder entries that will be
		// populated with real names on the next refresh if names matter.
		existing := c.currentCache()
		for _, id := range c.explicitIDs {
			if sc, ok := existing[id]; ok {
				services = append(services, sc.service)
			} else {
				services = append(services, api.Service{ID: id, Name: id})
			}
		}
	} else {
		var err error
		services, err = c.client.ListServices(ctx)
		if err != nil {
			return err
		}
	}

	c.mu.Lock()
	// Preserve existing data for services that are still present.
	next := make(map[string]serviceCache, len(services))
	for _, svc := range services {
		if existing, ok := c.cache[svc.ID]; ok {
			existing.service = svc // update name/version
			next[svc.ID] = existing
		} else {
			next[svc.ID] = serviceCache{service: svc}
		}
	}
	c.cache = next
	c.mu.Unlock()

	c.servicesCount.Set(float64(len(services)))
	return nil
}

// scrapeAll fetches the last complete 1-minute stats window for every service
// currently in the cache. It uses a semaphore to limit concurrency to 10.
func (c *Collector) scrapeAll(ctx context.Context) error {
	serviceIDs := c.currentServiceIDs()
	if len(serviceIDs) == 0 {
		return nil
	}

	start := time.Now()
	now := time.Now()
	from := now.Add(-660 * time.Second)
	to := now.Add(-60 * time.Second)

	type result struct {
		id   string
		data []api.StatsData
		err  error
	}

	sem := make(chan struct{}, 10)
	results := make(chan result, len(serviceIDs))

	for _, id := range serviceIDs {
		sem <- struct{}{}
		go func(serviceID string) {
			defer func() { <-sem }()
			data, err := c.client.GetStats(ctx, serviceID, from, to)
			results <- result{id: serviceID, data: data, err: err}
		}(id)
	}

	var firstErr error
	for range serviceIDs {
		r := <-results
		if r.err != nil {
			c.scrapeErrors.Inc()
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		c.mu.Lock()
		if sc, ok := c.cache[r.id]; ok {
			sc.data = r.data
			c.cache[r.id] = sc
		}
		c.mu.Unlock()
	}

	c.scrapeDuration.Set(time.Since(start).Seconds())
	if firstErr == nil {
		c.lastScrapeTime.Set(float64(time.Now().Unix()))
	}
	return firstErr
}

func (c *Collector) currentCache() map[string]serviceCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]serviceCache, len(c.cache))
	for k, v := range c.cache {
		cp[k] = v
	}
	return cp
}

func (c *Collector) currentServiceIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.cache))
	for id := range c.cache {
		ids = append(ids, id)
	}
	return ids
}
