// Package exporter ...
package exporter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ghouscht/metrics-server-exporter/internal/scrape"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

// API holds the exporter servers data.
type API struct {
	*http.ServeMux

	// k8s rest config
	cfg *rest.Config

	// context to cancel goroutines
	ctx context.Context

	// scrape intervals
	nodeScrapeInterval    *time.Ticker
	metricsScrapeInterval *time.Ticker
	scrapeTimeout         time.Duration

	l *zap.SugaredLogger
}

// Opt is a functional argument for the exporter server.
type Opt func(*API) error

// New returns an instance of the exporter server handler.
func New(ctx context.Context, cfg *rest.Config, options ...Opt) (*API, error) {
	api := API{
		ServeMux:              http.NewServeMux(),
		cfg:                   cfg,
		ctx:                   ctx,
		nodeScrapeInterval:    time.NewTicker(time.Hour * 1),
		metricsScrapeInterval: time.NewTicker(time.Second * 30),
		scrapeTimeout:         time.Second * 10,
		l:                     zap.New(nil).Sugar(), // noop logger
	}

	for _, opt := range options {
		if err := opt(&api); err != nil {
			return nil, err
		}
	}

	scraper, err := scrape.New(api.l, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating k8s scraper client: %w", err)
	}

	// initially scrape nodes and metrics-server to make sure we have data
	ctx, cancel := context.WithTimeout(api.ctx, api.scrapeTimeout)
	defer cancel()

	if err := scraper.Nodes(ctx); err != nil {
		return nil, fmt.Errorf("reading k8s node resources: %w", err)
	}

	ctx, cancel = context.WithTimeout(api.ctx, api.scrapeTimeout)
	defer cancel()

	if err := scraper.MetricsServer(ctx); err != nil {
		return nil, fmt.Errorf("reading metrics from metris-server: %w", err)
	}

	go func() {
	loop:
		for {
			select {
			case <-api.nodeScrapeInterval.C:
				api.l.Debugf("scraping nodes")

				ctx, cancel := context.WithTimeout(api.ctx, api.scrapeTimeout)
				if err := scraper.Nodes(ctx); err != nil {
					api.l.Errorf("scraping nodes: %s", err)
				}

				cancel() // free resources
			case <-api.metricsScrapeInterval.C:
				api.l.Debugf("scraping metrics server")

				ctx, cancel := context.WithTimeout(api.ctx, api.scrapeTimeout)
				if err := scraper.MetricsServer(ctx); err != nil {
					api.l.Errorf("scraping metrics server: %s", err)
				}

				cancel() // free resources
			case <-api.ctx.Done():
				break loop
			}
		}
		api.l.Infof("scraping stopped")
	}()

	api.Handle("/metrics", promhttp.Handler())
	api.HandleFunc("/ready", api.ready())

	return &api, nil
}

// WithNodeScrapeInterval overrides the default interval in which k8s nodes are queried for their
// allocatable/capacity of resources.
func WithNodeScrapeInterval(interval time.Duration) Opt {
	return func(a *API) error {
		a.nodeScrapeInterval = time.NewTicker(interval)
		return nil
	}
}

// WithMetricsScrapeInterval overrides the default interval in which the experter will query the
// metrics-server for node and pod metrics.
func WithMetricsScrapeInterval(interval time.Duration) Opt {
	return func(a *API) error {
		a.metricsScrapeInterval = time.NewTicker(interval)
		return nil
	}
}

// WithExcludedNamespaces is used to exclude the given namespaces from metric collection.
func WithExcludedNamespaces(exclude []string) Opt {
	return func(a *API) error {
		return fmt.Errorf("not implemented")
	}
}

// WithLogger configures the exporter with a zap logger (default is a noop logger).
func WithLogger(l *zap.SugaredLogger) Opt {
	return func(a *API) error {
		a.l = l
		return nil
	}
}

// WithScrapeTimeout overrides the default timeout to scrape metrics from the metrics-server.
func WithScrapeTimeout(t time.Duration) Opt {
	return func(a *API) error {
		a.scrapeTimeout = t
		return nil
	}
}

func (a *API) ready() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}
