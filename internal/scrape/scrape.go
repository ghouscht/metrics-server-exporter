// Package scrape implements the functions to read (=> scrape) the apiserver for node/pod capacity
// and resource metrics.
package scrape

import (
	"context"
	"fmt"

	"github.com/ghouscht/metrics-server-exporter/internal/metrics"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	metricsGroupName = "metrics.k8s.io"
)

type nodeMetrics struct {
	Usage corev1.ResourceList
}

type podMetrics struct {
	Containers []struct {
		Usage corev1.ResourceList
		Name  string
	}
}

// Scraper is ...
type Scraper struct {
	cliset *kubernetes.Clientset
	dyn    dynamic.Interface

	l *zap.SugaredLogger
}

// New returns a new instance of Scraper, ready to use.
func New(l *zap.SugaredLogger, cfg *rest.Config) (*Scraper, error) {
	scraper := Scraper{
		l: l,
	}

	cliset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic: %w", err)
	}

	scraper.cliset = cliset
	scraper.dyn = dyn

	return &scraper, nil
}

// Nodes scrapes all k8s nodes and updates the node resource capacity/allocatable metrics.
func (s *Scraper) Nodes(ctx context.Context) error {
	nodeList, err := s.cliset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for idx := range nodeList.Items {
		metrics.SetNodeResourceCapacity(
			nodeList.Items[idx].GetName(),
			metrics.CPU,
			float64(nodeList.Items[idx].Status.Allocatable.Cpu().MilliValue()),
		)
		metrics.SetNodeResourceCapacity(
			nodeList.Items[idx].GetName(),
			metrics.Memory,
			float64(nodeList.Items[idx].Status.Allocatable.Memory().ScaledValue(resource.Kilo)),
		)
	}

	return nil
}

// MetricsServer scrapes the metrics-server for node and pod usage information and updates the metrics.
//nolint:funlen,gocognit,gocyclo
func (s *Scraper) MetricsServer(ctx context.Context) error {
	groups, err := s.cliset.Discovery().ServerGroups()
	if err != nil {
		return fmt.Errorf("get discovered api server groups: %w", err)
	}

	// get the preffered group version so we can later discover the resources
	var groupVersion string
	//nolint:gocritic
	for _, group := range groups.Groups {
		if group.Name == metricsGroupName {
			groupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}

	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return fmt.Errorf("parsing group version %q: %w", groupVersion, err)
	}

	resources, err := s.cliset.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	//nolint:gocritic
	for _, r := range resources.APIResources {
		rinterface := s.dyn.Resource(gv.WithResource(r.Name))

		if r.Namespaced {
			list, err := rinterface.List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			items, err := meta.ExtractList(list)
			if err != nil {
				return err
			}

			for _, item := range items {
				unstructured, ok := item.(runtime.Unstructured)
				if !ok {
					s.l.Error("asserting to interface 'runtime.Unstructured' failed")
					continue
				}

				metadata, err := meta.Accessor(unstructured)
				if err != nil {
					s.l.Errorf("extracting metadata from object: %s", err)
					continue
				}

				var pm podMetrics
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &pm); err != nil {
					s.l.Errorf("conversion from unstructured to object: %s", err)
					continue
				}

				for _, container := range pm.Containers {
					metrics.SetPodResourceUsage(
						metadata.GetNamespace(),
						metadata.GetName(),
						metrics.CPU,
						float64(container.Usage.Cpu().MilliValue()),
					)

					metrics.SetPodResourceUsage(
						metadata.GetNamespace(),
						metadata.GetName(),
						metrics.Memory,
						float64(container.Usage.Memory().MilliValue()),
					)
				}
			}
		} else {
			list, err := rinterface.List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			items, err := meta.ExtractList(list)
			if err != nil {
				return err
			}

			for _, item := range items {
				unstructured, ok := item.(runtime.Unstructured)
				if !ok {
					s.l.Error("asserting to interface 'runtime.Unstructured' failed")
					continue
				}

				metadata, err := meta.Accessor(unstructured)
				if err != nil {
					s.l.Errorf("extracting metadata from object: %s", err)
					continue
				}

				var nm nodeMetrics
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &nm); err != nil {
					s.l.Errorf("conversion from unstructured to object: %s", err)
					continue
				}

				metrics.SetNodeResourceUsage(metadata.GetName(), metrics.CPU, float64(nm.Usage.Cpu().MilliValue()))
				metrics.SetNodeResourceUsage(metadata.GetName(), metrics.Memory, float64(nm.Usage.Memory().ScaledValue(resource.Kilo)))
			}
		}
	}

	return err
}
