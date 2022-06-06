package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jmanero/bitcoind-exporter/pkg/bitcoind"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/btcsuite/btcd/rpcclient"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// CLI Flag Values
var (
	// Exporter Endpoint
	ListenFlag          string
	ExportPathFlag      string
	ShutdownTimeoutFlag time.Duration

	// bitcoind Connection Configuration
	Config rpcclient.ConnConfig
)

var logger, _ = zap.NewProduction()
var registry = prometheus.NewRegistry()
var router = http.NewServeMux()

func init() {
	pflag.StringVar(&ListenFlag, "listen", "0.0.0.0:9142", "Bind address/port for HTTP exporter service")
	pflag.StringVar(&ExportPathFlag, "export-path", "/metrics", "HTTP endpoint for prometheus metrics")
	pflag.DurationVar(&ShutdownTimeoutFlag, "shutdown-timeout", 15*time.Second, "Timeout for HTTP service shutdown")

	// Configure the RPC client
	pflag.StringVar(&Config.Host, "rpc-addr", "127.0.0.1:8332", "RPC address")
	pflag.BoolVar(&Config.DisableTLS, "no-rpc-tls", false, "Disable TLS on RPC connections")
	pflag.BoolVar(&Config.HTTPPostMode, "rpc-http-post", false, "Use HTTP POST method for RPC requests")
	pflag.StringVar(&Config.User, "rpc-user", "", "RPC authentication user")
	pflag.StringVar(&Config.Pass, "rpc-pass", "", "RPC authentication password")
	pflag.StringVar(&Config.CookiePath, "rpc-cookie", "", "RPC authentication cookie file path")

	// Configure baseline collectors for go program monitoring
	registry.MustRegister(
		collectors.NewGoCollector(collectors.WithGoCollections(collectors.GoRuntimeMetricsCollection)),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// Main program. Calling os.Exit to set an error code fails to execute deferred functions.
func Main() int {
	pflag.Parse()

	// Trap shutdown signals to ensure that the program will behave when run as PID1
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	// Connect to bitcoind RPC service
	logger.Info("Connecting to RPC service", zap.String("addr", Config.Endpoint), zap.Bool("tls", !Config.DisableTLS), zap.Bool("http-post", Config.HTTPPostMode))
	client, err := rpcclient.New(&Config, &rpcclient.NotificationHandlers{})
	if err != nil {
		logger.Error("Unable to create RPC client", zap.Error(err))
		return 1
	}

	// Create bitcoind collectors
	bc := &bitcoind.BlockchainCollector{Client: client, Logger: logger.Named("collector.blockchain")}
	err = registry.Register(bc)
	if err != nil {
		logger.Error("Unable to create bitcoind RPC collector", zap.Error(err))
		return 1
	}

	// Setup exporter endpoint
	logger.Info("Handling prometheus metrics", zap.String("path", ExportPathFlag))
	opts := promhttp.HandlerOpts{EnableOpenMetrics: true}
	opts.ErrorLog, _ = zap.NewStdLogAt(logger.Named("prometheus"), zap.ErrorLevel)
	router.Handle(ExportPathFlag, promhttp.HandlerFor(registry, opts))

	logger.Info("Creating listener", zap.String("addr", ListenFlag))
	listener, err := net.Listen("tcp", ListenFlag)
	if err != nil {
		logger.Error("Unable to create listener", zap.Error(err))
		return 1
	}

	server := http.Server{Handler: router}
	server.ErrorLog, _ = zap.NewStdLogAt(logger.Named("http"), zap.ErrorLevel)

	logger.Info("Starting HTTP service")
	errs := make(chan error)
	go func() { errs <- server.Serve(listener) }()

	select {
	case <-ctx.Done():
		logger.Info("Shutting down", zap.Duration("timeout", ShutdownTimeoutFlag))

		// Set up a new signal listener to force-exit
		ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
		ctx, done := context.WithTimeout(ctx, ShutdownTimeoutFlag)
		defer done()

		err = server.Shutdown(ctx)
		if err != nil {
			logger.Error("Shutdown error", zap.Error(err))
			return 1
		}
	case err = <-errs:
		logger.Error("HTTP server error", zap.Error(err))
		return 1
	}

	logger.Info("Goodbye")
	return 0
}

func main() {
	os.Exit(Main())
}
