// Package main is the entry point for the Fastly historical stats Prometheus exporter.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/api"
	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/exporter"
	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/filter"
)

var programVersion = "dev"

// stringSlice implements flag.Value for repeatable string flags.
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ok")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<html><head><title>Fastly Historical Stats Exporter</title></head>
<body>
<h1>Fastly Historical Stats Exporter</h1>
<p>Version: %s</p>
<p><a href="/metrics">Metrics</a></p>
<p><a href="/healthz">Health</a></p>
</body></html>`, programVersion)
}

func main() {
	var (
		serviceIDs      stringSlice
		token           string
		listenAddr      string
		metricAllowlist string
		metricBlocklist string
		namespace       string
		scrapeInterval  time.Duration
		refreshInterval time.Duration
	)

	flag.StringVar(&token, "token", "", "Fastly API token (overridden by FASTLY_API_TOKEN env var if not set)")
	flag.Var(&serviceIDs, "service", "Explicit Fastly service ID to export (repeatable; default: all services)")
	flag.StringVar(&listenAddr, "listen", ":8080", "TCP address to listen on")
	flag.StringVar(&metricAllowlist, "metric-allowlist", "", "Regex; only export metrics whose name matches (e.g. 'bytes$')")
	flag.StringVar(&metricBlocklist, "metric-blocklist", "", "Regex; exclude metrics whose name matches (e.g. 'imgopto')")
	flag.StringVar(&namespace, "namespace", "fastly", "Prometheus metric namespace prefix")
	flag.DurationVar(&scrapeInterval, "scrape-interval", 60*time.Second, "How often to fetch stats from the Fastly API")
	flag.DurationVar(&refreshInterval, "refresh-interval", 5*time.Minute, "How often to refresh the list of Fastly services")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of fastly-historical-stats-exporter (version %s):\n\n", programVersion)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  FASTLY_API_TOKEN  Fastly API token (used if -token flag is not set)\n")
	}

	flag.Parse()

	// Token: flag takes precedence over environment variable.
	if token == "" {
		token = os.Getenv("FASTLY_API_TOKEN")
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: a Fastly API token is required; use -token or FASTLY_API_TOKEN")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Build metric filter from allowlist/blocklist flags.
	mf := &filter.Filter{}
	if metricAllowlist != "" {
		if err := mf.Allow(metricAllowlist); err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid -metric-allowlist: %v\n", err)
			os.Exit(1)
		}
	}
	if metricBlocklist != "" {
		if err := mf.Block(metricBlocklist); err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid -metric-blocklist: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := api.NewClient(token)

	collector := exporter.NewCollector(
		ctx,
		client,
		mf,
		namespace,
		scrapeInterval,
		refreshInterval,
		[]string(serviceIDs),
		logger,
	)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/", rootHandler)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info(
			"starting exporter",
			"version", programVersion,
			"listen", listenAddr,
			"scrape_interval", scrapeInterval,
			"refresh_interval", refreshInterval,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}
