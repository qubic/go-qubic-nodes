package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ardanlabs/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qubic/go-qubic-nodes/metrics"
	"github.com/qubic/go-qubic-nodes/node"
	"github.com/qubic/go-qubic-nodes/web"
)

const prefix = "QUBIC_NODES"

type Configuration struct {
	Qubic struct {
		PeerList                 []string      `conf:"default:5.39.222.64;82.197.173.130;82.197.173.129"`
		PeerPort                 string        `conf:"default:21841"`
		ExchangeTimeout          time.Duration `conf:"default:2s"`
		MaxTickErrorThreshold    uint32        `conf:"default:50"`
		ReliableTickRange        uint32        `conf:"default:30"`
		UsePublicPeers           bool          `conf:"default:false"`
		PublicPeersExclude       []string
		PublicPeersCleanInterval time.Duration `conf:"default:24h"`
	}
	Service struct {
		TickerUpdateInterval time.Duration `conf:"default:15s"`
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

	peerDiscovery := createPeerDiscoveryStrategy(config)
	peerManager := node.NewPeerManager(config.Qubic.PeerList, peerDiscovery, config.Qubic.PeerPort, config.Qubic.ExchangeTimeout)
	container, err := node.NewNodeContainer(peerManager, config.Qubic.MaxTickErrorThreshold, config.Qubic.ReliableTickRange, m)
	if err != nil {
		log.Printf("Error: %v\n", err)
	}

	go func() {
		ticker := time.NewTicker(config.Service.TickerUpdateInterval)

		for {
			select {
			case <-ticker.C:
				updateErr := container.Update()
				if updateErr != nil {
					log.Printf("Error: %v\n", updateErr)
				}
			}
		}
	}()

	log.Printf("Staring WebServer...\n")

	handler := web.PeersHandler{
		Container: container,
	}

	router := http.NewServeMux()

	router.HandleFunc("GET /status", handler.HandleStatus)
	router.HandleFunc("GET /max-tick", handler.HandleMaxTick)
	router.HandleFunc("POST /reliable-nodes", handler.GetReliableNodesWithMinimumTick)
	router.Handle("/metrics", promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	return http.ListenAndServe(":8080", router)

}

func createPeerDiscoveryStrategy(config Configuration) node.PeerDiscovery {
	if config.Qubic.UsePublicPeers {
		log.Println("main: Using public peers")
		return node.NewPublicPeerDiscovery(config.Qubic.PeerPort, config.Qubic.ExchangeTimeout, config.Qubic.PublicPeersExclude, config.Qubic.PublicPeersCleanInterval)
	} else {
		log.Println("main: Using static peers")
		return &node.NoPeerDiscovery{}
	}
}
