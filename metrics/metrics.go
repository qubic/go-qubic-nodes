package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type NodesServiceMetrics struct {
	prometheusRegistry *prometheus.Registry

	configuredNodeCount prometheus.Gauge
	reliableNodeCount   prometheus.Gauge
	networkTick         prometheus.Gauge
}

func NewNodesServiceMetrics(registry *prometheus.Registry, namespace string) *NodesServiceMetrics {
	factory := promauto.With(registry)
	metrics := NodesServiceMetrics{
		prometheusRegistry: registry,

		configuredNodeCount: factory.NewGauge(prometheus.GaugeOpts{
			Name: namespace + "_configured_node_count",
			Help: "The number of configured nodes.",
		}),
		reliableNodeCount: factory.NewGauge(prometheus.GaugeOpts{
			Name: namespace + "_reliable_node_count",
			Help: "The number of currently reliable nodes.",
		}),
		networkTick: factory.NewGauge(prometheus.GaugeOpts{
			Name: namespace + "_network_tick",
			Help: "The current network tick.",
		}),
	}
	return &metrics
}

func (m *NodesServiceMetrics) SetConfiguredNodeCount(nodeCount int) {
	m.configuredNodeCount.Set(float64(nodeCount))
}

func (m *NodesServiceMetrics) SetReliableNodeCount(nodeCount int) {
	m.reliableNodeCount.Set(float64(nodeCount))
}

func (m *NodesServiceMetrics) SetNetworkTick(tickNumber uint32) {
	m.networkTick.Set(float64(tickNumber))
}
