package main

import (
	"fmt"
	"github.com/ardanlabs/conf"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qubic/go-qubic-nodes/node"
	"github.com/qubic/go-qubic-nodes/web"
	"log"
	"net/http"
	"os"
	"time"
)

const prefix = "QUBIC_NODES"

type Configuration struct {
	Qubic struct {
		PeerList              []string      `conf:"default:5.39.222.64;82.197.173.130;82.197.173.129"`
		PeerPort              string        `conf:"default:21841"`
		ExchangeTimeout       time.Duration `conf:"default:2s"`
		MaxTickErrorThreshold uint32        `conf:"default:50"`
		ReliableTickRange     uint32        `conf:"default:30"`
	}
	Service struct {
		TickerUpdateInterval time.Duration `conf:"default:5s"`
		MetricsInstanceLabel string
		MetricsAddress       string `conf:"default:0.0.0.0:2112"`
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
				return errors.Wrap(err, "generating config usage")
			}
			fmt.Println(usage)
			return nil
		case conf.ErrVersionWanted:
			version, err := conf.VersionString(prefix, &config)
			if err != nil {
				return errors.Wrap(err, "generating config version")
			}
			fmt.Println(version)
			return nil
		}
		return errors.Wrap(err, "parsing config")
	}
	out, err := conf.String(&config)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	log.Printf("main: Config :\n%v\n", out)

	createMetricsGauges(config.Service.MetricsInstanceLabel)
	totalConfiguredNodes.Set(float64(len(config.Qubic.PeerList)))

	container, err := node.NewNodeContainer(config.Qubic.PeerList, config.Qubic.PeerPort, config.Qubic.MaxTickErrorThreshold, config.Qubic.ReliableTickRange, config.Qubic.ExchangeTimeout)
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
					continue
				}

				response := container.GetResponse()
				reliableNodes.Set(float64(len(response.ReliableNodes)))

			}
		}
	}()

	log.Printf("Staring WebServer...\n")

	handler := web.RequestHandler{
		Container: container,
	}

	http.HandleFunc("/status", handler.HandleStatus)
	http.HandleFunc("/max-tick", handler.HandleMaxTick)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.InstrumentMetricHandler(registry, promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))

		metricsServer := http.Server{
			Addr:    config.Service.MetricsAddress,
			Handler: mux,
		}
		err := metricsServer.ListenAndServe()
		if err != nil {
			log.Printf("Metrics server failed: %s\n", err)
		}
	}()

	return http.ListenAndServe(":8080", nil)

}

var (
	registry = prometheus.NewRegistry()
	factory  = promauto.With(registry)

	reliableNodes        prometheus.Gauge
	totalConfiguredNodes prometheus.Gauge
)

func createMetricsGauges(instanceLabel string) {

	var labels prometheus.Labels

	if len(instanceLabel) != 0 {
		labels = make(prometheus.Labels)
		labels["name"] = instanceLabel
	}

	reliableNodes = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "qubic_nodes_reliable_node_count",
		Help:        "The number of current reliable nodes.",
		ConstLabels: labels,
	})
	totalConfiguredNodes = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "qubic_nodes_configured_node_count",
		Help:        "The number of total configured nodes.",
		ConstLabels: labels,
	})

}
