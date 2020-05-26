// Package metrics contains the metric definitions which will be exported by metrics-server-exporter.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricsNamespace = "metrics_server_exporter"
)

// Resource is a type to describe a compute resource
type Resource int

const (
	// CPU describes cpu resources
	CPU Resource = iota
	// Memory describes memory resources
	Memory
)

func (r Resource) String() string {
	return [...]string{"cpu", "memory"}[r]
}

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

	podResourceUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: "pod",
		Name:      "resource_usage",
		Help:      "blabla",
	}, []string{"namespace", "pod", "resource"})
)

// SetNodeResourceUsage sets the resource usage metric for the given node and compute resource.
func SetNodeResourceUsage(node string, r Resource, value float64) {
	nodeResourceUsage.WithLabelValues(
		node,
		r.String(),
	).Set(value)
}

// SetNodeResourceCapacity sets the actual capacity metric for a node and compute resource.
func SetNodeResourceCapacity(node string, r Resource, value float64) {
	nodeResourceCapacity.WithLabelValues(
		node,
		r.String(),
	).Set(value)
}

// SetPodResourceUsage sets the resource usage metric for the given namespace/pod combination.
func SetPodResourceUsage(namespace, pod string, r Resource, value float64) {
	podResourceUsage.WithLabelValues(
		namespace,
		pod,
		r.String(),
	).Set(value)
}
