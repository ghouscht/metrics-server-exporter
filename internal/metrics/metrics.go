package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricsNamespace = "metrics_server_exporter"
)

var (
	nodeResourceUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: "node",
		Name:      "resource_usage",
		Help:      "blabla",
	}, []string{"node", "resource"})

	nodeResourceCapacity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: "node",
		Name:      "resource_capacity",
		Help:      "blabla",
	}, []string{"node", "resource"})
)

func SetNodeResourceUsage(node string, resource string, value float64) {
	nodeResourceUsage.WithLabelValues(
		node,
		resource,
	).Set(value)
}

func SetNodeResourceCapacity(node string, resource string, value float64) {
	nodeResourceCapacity.WithLabelValues(
		node,
		resource,
	).Set(value)
}
