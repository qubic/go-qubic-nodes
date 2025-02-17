package web

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type MetricsServer struct {
	address string
}

func NewMetricsServer(address string) *MetricsServer {
	server := &MetricsServer{
		address: address,
	}
	return server
}

func (s *MetricsServer) Start() {

	go func() {
		serverMux := http.NewServeMux()
		serverMux.Handle("/metrics", promhttp.Handler())

		var server = &http.Server{
			Addr:              s.address,
			Handler:           serverMux,
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 15 * time.Second,
			WriteTimeout:      15 * time.Second,
		}

		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
}
