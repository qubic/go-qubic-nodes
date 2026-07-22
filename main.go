package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qubic/go-qubic-nodes/metrics"
	"github.com/qubic/go-qubic-nodes/peer"
	"github.com/qubic/go-qubic-nodes/web"
)

const prefix = "QUBIC_NODES"

type Configuration struct {
	Qubic struct {
		PeerList                  []string      `conf:"default:5.39.222.64;82.197.173.130;82.197.173.129"`
		PeerPort                  string        `conf:"default:21841"`
		PeerTimeout               time.Duration `conf:"default:3s"`
		PeerCap                   int           `conf:"default:10"`
		PeerSyncThreshold         uint32        `conf:"default:30"`
		PeerResponseTimeThreshold time.Duration `conf:"default:250ms"`
	}
	Service struct {
		EnableDiscovery bool          `conf:"default:true"`
		UpdateInterval  time.Duration `conf:"default:15s"`
		Debug           bool          `conf:"default:false"`
		FastWarmup      bool          `conf:"default:true"`
	}
	Metrics struct {
		Namespace string `conf:"default:qubic_nodes"`
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("main: exited with error: %s\n", err.Error())
	}
}

func run() error {

	var config Configuration

	if err := conf.Parse(os.Args[1:], prefix, &config); err != nil {
		switch err {
		case conf.ErrHelpWanted:
			usage, err := conf.Usage(prefix, &config)
			if err != nil {
				return fmt.Errorf("generating config usage: %w", err)
			}
			fmt.Println(usage)
			return nil
		case conf.ErrVersionWanted:
			version, err := conf.VersionString(prefix, &config)
			if err != nil {
				return fmt.Errorf("generating config version: %w", err)
			}
			fmt.Println(version)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}
	out, err := conf.String(&config)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Printf("main: Config :\n%v\n", out)

	prometheusRegistry := prometheus.NewRegistry()
	prometheusRegistry.MustRegister(collectors.NewGoCollector())
	m := metrics.NewNodesServiceMetrics(prometheusRegistry, config.Metrics.Namespace)

	managerConfig := peer.ManagerConfig{
		EnableDiscovery:                     config.Service.EnableDiscovery,
		UpdateInterval:                      config.Service.UpdateInterval,
		Debug:                               config.Service.Debug,
		FastWarmUp:                          config.Service.FastWarmup,
		SeedPeers:                           config.Qubic.PeerList,
		PeerPort:                            config.Qubic.PeerPort,
		PeerInfoTimeout:                     config.Qubic.PeerTimeout,
		MaxPeers:                            config.Qubic.PeerCap,
		NetworkTickAcceptanceThreshold:      config.Qubic.PeerSyncThreshold,
		PeerResponseTimeAcceptanceThreshold: config.Qubic.PeerResponseTimeThreshold,
	}
	peerManager := peer.NewPeerManager(managerConfig, m)

	// Root context cancelled on SIGINT/SIGTERM to coordinate a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go peerManager.Start(ctx)

	log.Printf("Staring WebServer...\n")

	handler := web.PeersHandler{
		PeerManager: peerManager,
	}

	router := http.NewServeMux()

	router.HandleFunc("GET /status", handler.HandleStatus)
	router.HandleFunc("GET /max-tick", handler.HandleMaxTick)
	router.HandleFunc("POST /reliable-nodes", handler.GetReliableNodesWithMinimumTick)
	router.Handle("/metrics", promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{EnableOpenMetrics: true}))

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Run the server in its own goroutine so we can wait for the shutdown signal.
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	// Wait for either the server to fail or a shutdown signal.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Printf("main: shutdown signal received, stopping...\n")
	}

	// Give in-flight requests time to complete before forcing the server down.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down web server: %w", err)
	}

	log.Printf("main: shutdown complete\n")
	return nil
}
