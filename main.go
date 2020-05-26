package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/ghouscht/metrics-server-exporter/internal/exporter"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// load all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

//nolint:gochecknoglobals
var (
	// set by goreleaser on build
	version, date, commit string = "master", "?", "?"
	// name of the binary
	binaryName = filepath.Base(os.Args[0])
)

//nolint:gochecknoglobals
var (
	inCluster             = flag.Bool("in-cluster", true, "Run with kubernetes in-cluster config")
	debug                 = flag.Bool("debug", false, "Enable debug loging")
	listen                = flag.String("listen", ":8080", "Address where the server should listen for requests")
	ver                   = flag.Bool("version", false, "Print version and exit")
	nodeScrapeInterval    = flag.Duration("node-scrape-interval", time.Hour*1, "Interval how often to scrape node capacity/allocatable resources")
	metricsScrapeInterval = flag.Duration("metrics-scrape-interval", time.Second*30, "Interval how often to scrape metrics-server metrics (node and pod metrics)")
)

func main() {
	var (
		// default to info loglevel
		logLevel zapcore.Level = zap.InfoLevel
	)

	flag.Parse()

	if *debug {
		logLevel = zap.DebugLevel
	}

	if *ver {
		printVersion()
		return
	}

	// zap setup
	atom := zap.NewAtomicLevelAt(logLevel)
	config := zap.NewProductionConfig()
	config.DisableStacktrace = true
	config.Sampling = nil
	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	config.Level = atom

	zl, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("zap logger creation: %s", err))
	}

	l := zl.Sugar()

	l.Infow("starting server", "version", version, "listen", *listen, "loglevel", atom.Level())

	runServer(l, k8sRestConfig(l))
}

func k8sRestConfig(l *zap.SugaredLogger) *rest.Config {
	var cfg *rest.Config

	if *inCluster {
		var err error

		cfg, err = rest.InClusterConfig()
		if err != nil {
			l.Fatalf("in-cluster config creation: %s", err)
		}
	} else {
		var err error

		cfg, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			l.Fatalf("config creation: %s", err)
		}
	}

	cfg.UserAgent = fmt.Sprintf("%s/%s (%s/%s) ", binaryName, version, goruntime.GOOS, goruntime.GOARCH)

	return cfg
}

func runServer(l *zap.SugaredLogger, cfg *rest.Config) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	ctx, cancel := context.WithCancel(context.Background())

	handler, err := exporter.New(ctx, cfg,
		exporter.WithNodeScrapeInterval(*nodeScrapeInterval),
		exporter.WithMetricsScrapeInterval(*metricsScrapeInterval),
		exporter.WithLogger(l),
	)
	if err != nil {
		l.Fatalf("creating exporter: %s", err)
	}

	server := http.Server{
		Addr:    *listen,
		Handler: handler,
	}

	// as soon as a signal is received cancel the context to allow a graceful stop of all other
	// components
	go func() {
		sig := <-stop

		cancel()
		l.Infof("stopping execution, received signal %q", sig)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			l.Fatalf("shutdown server: %s", err)
		}

		l.Debug("gracefully stopped server")
	}()

	if err := server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			l.Fatalf("server: %s", err)
		}
	}
}

func printVersion() {
	fmt.Printf("%s, version %s (revision: %s)\n\tbuild date: %s\n\tgo version: %s\n",
		binaryName,
		version,
		commit,
		date,
		goruntime.Version(),
	)
}
