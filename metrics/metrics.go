package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	totalConfiguredNodesGauge prometheus.Gauge
	reliableNodesGauge        prometheus.Gauge
}

func NewMetrics() *Metrics {
	return &Metrics{
		totalConfiguredNodesGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "qubic_nodes_configured_nodes",
			Help: "The number of total configured nodes.",
		}),
		reliableNodesGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "qubic_nodes_reliable_nodes",
			Help: "The number of current reliable nodes.",
		}),
	}
}

func (m *Metrics) SetTotalConfiguredNodes(count int) {
	m.totalConfiguredNodesGauge.Set(float64(count))
}

func (m *Metrics) SetReliableNodes(count int) {
	m.reliableNodesGauge.Set(float64(count))
}
