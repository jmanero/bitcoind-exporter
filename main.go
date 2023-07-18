package main

import (
	"context"
	"fmt"
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
	"go.uber.org/zap/zapcore"
)

// CLI Flag Values
var (
	listenFlag          string
	exportPathFlag      string
	shutdownTimeoutFlag time.Duration
	logLevelFlag        string

	// bitcoind Connection Configuration
	config rpcclient.ConnConfig
)

var registry = prometheus.NewRegistry()
var router = http.NewServeMux()
var logger *zap.Logger
var client *rpcclient.Client

func init() {
	pflag.StringVar(&listenFlag, "listen", "0.0.0.0:9142", "Bind address/port for HTTP exporter service")
	pflag.StringVar(&exportPathFlag, "export-path", "/metrics", "HTTP endpoint for prometheus metrics")
	pflag.DurationVar(&shutdownTimeoutFlag, "shutdown-timeout", 15*time.Second, "Timeout for HTTP service shutdown")
	pflag.StringVar(&logLevelFlag, "log-level", "info", "Logging output level")

	// Configure the RPC client
	pflag.StringVar(&config.Host, "rpc-addr", "127.0.0.1:8332", "RPC address")
	pflag.BoolVar(&config.DisableTLS, "no-rpc-tls", false, "Disable TLS on RPC connections")
	pflag.BoolVar(&config.HTTPPostMode, "rpc-http-post", false, "Use HTTP POST method for RPC requests")
	pflag.StringVar(&config.User, "rpc-user", "", "RPC authentication user")
	pflag.StringVar(&config.Pass, "rpc-pass", "", "RPC authentication password")
	pflag.StringVar(&config.CookiePath, "rpc-cookie", "", "RPC authentication cookie file path")

	// Configure baseline collectors for go program monitoring
	registry.MustRegister(
		collectors.NewGoCollector(collectors.WithGoCollections(collectors.GoRuntimeMetricsCollection)),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// Logger initializes a logger for the service
func Logger() error {
	level, err := zapcore.ParseLevel(logLevelFlag)
	if err != nil {
		return err
	}

	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), level)

	logger = zap.New(core)
	return nil
}

// RPCClient initializes a JSON-RPC client for collectors
func RPCClient() (err error) {
	// Connect to bitcoind RPC service
	logger.Info("Connecting to RPC service", zap.String("addr", config.Host), zap.Bool("tls", !config.DisableTLS), zap.Bool("http-post", config.HTTPPostMode))
	client, err = rpcclient.New(&config, nil)
	if err != nil {
		return
	}

	return client.Ping()
}

// Serve the exporter HTTP endpoint
func Serve(ctx context.Context) error {
	logger := logger.Named("http")

	logger.Info("Creating listener", zap.String("addr", listenFlag))
	listener, err := net.Listen("tcp", listenFlag)
	if err != nil {
		logger.Error("Unable to create listener", zap.Error(err))
		return err
	}

	server := http.Server{Handler: router}
	server.ErrorLog, _ = zap.NewStdLogAt(logger.Named("server"), zap.ErrorLevel)

	logger.Info("Starting HTTP service")
	errs := make(chan error)
	go func() { errs <- server.Serve(listener) }()

	select {
	case <-ctx.Done():
		logger.Info("Shutting down", zap.Duration("timeout", shutdownTimeoutFlag))

		// Set up a new signal listener to force-exit
		ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
		ctx, done := context.WithTimeout(ctx, shutdownTimeoutFlag)
		defer done()

		err = server.Shutdown(ctx)
		if err != nil {
			logger.Error("Shutdown error", zap.Error(err))
			return err
		}
	case err = <-errs:
		logger.Error("Accept error", zap.Error(err))
		return err
	}

	return nil
}

// Main program. Calling os.Exit to set an error code fails to execute deferred functions.
func Main() int {
	pflag.Parse()

	err := Logger()
	if err != nil {
		fmt.Println("Unable to configure logger:", err)
		return 1
	}

	err = RPCClient()
	if err != nil {
		logger.Error("Unable to create RPC client", zap.String("addr", config.Endpoint), zap.Error(err))
		return 1
	}

	// Trap shutdown signals to ensure that the program will behave when run as PID1
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	// Create bitcoind collectors
	logger.Info("Registering bitcoind_blockchain collector")
	err = registry.Register(bitcoind.NewBlockchainCollector(client, logger.Named("collector.bitcoind.blockchain")))
	if err != nil {
		logger.Error("Unable to create bitcoind.BlockchainCollector", zap.Error(err))
		return 1
	}

	logger.Info("Registering bitcoind_mempool collector")
	err = registry.Register(bitcoind.NewMempoolCollector(client, logger.Named("collector.bitcoind.mempool")))
	if err != nil {
		logger.Error("Unable to create bitcoind.MempoolCollector", zap.Error(err))
		return 1
	}

	logger.Info("Registering bitcoind_peer collector")
	err = registry.Register(bitcoind.NewPeersCollector(client, logger.Named("collector.bitcoind.peers")))
	if err != nil {
		logger.Error("Unable to create bitcoind.PeersCollector", zap.Error(err))
		return 1
	}

	logger.Info("Registering bitcoind_index collector")
	err = registry.Register(bitcoind.NewIndexCollector(client, logger.Named("collector.bitcoind.index")))
	if err != nil {
		logger.Error("Unable to create bitcoind.IndexCollector", zap.Error(err))
		return 1
	}

	// Setup exporter endpoint
	logger.Info("Handling prometheus metrics", zap.String("path", exportPathFlag))
	opts := promhttp.HandlerOpts{EnableOpenMetrics: true}
	opts.ErrorLog, _ = zap.NewStdLogAt(logger.Named("exporter.handler"), zap.ErrorLevel)
	router.Handle(exportPathFlag, promhttp.HandlerFor(registry, opts))

	err = Serve(ctx)
	if err != nil {
		return 1
	}

	logger.Info("Goodbye")
	return 0
}

func main() {
	os.Exit(Main())
}
