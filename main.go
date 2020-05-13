package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/ghouscht/metrics-server-exporter/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// load all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	inCluster = flag.Bool("in-cluster", true, "Run with kubernetes in-cluster config")
)

const (
	metricsGroupName = "metrics.k8s.io"
)

type Metrics struct {
	Usage corev1.ResourceList
}

func main() {
	flag.Parse()

	var cfg *rest.Config

	if *inCluster {
		var err error
		cfg, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var err error
		cfg, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			log.Fatal(err)
		}
	}

	cfg.UserAgent = fmt.Sprintf("%s/%s (%s/%s) ", "metrics-server-exporter", "v0.0.1", goruntime.GOOS, goruntime.GOARCH)

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	cliset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	groups, err := cliset.Discovery().ServerGroups()
	if err != nil {
		log.Fatal(err)
	}

	// get the preffered group version so we can later discover the resources
	var groupVersion string
	for _, group := range groups.Groups {
		if group.Name == metricsGroupName {
			groupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}

	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		log.Fatal(err)
	}

	resources, err := cliset.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range resources.APIResources {
		rinterface := dyn.Resource(gv.WithResource(r.Name))

		if r.Namespaced {
			// TODO
		} else {
			list, err := rinterface.List(context.Background(), metav1.ListOptions{})
			if err != nil {
				log.Fatal(err)
			}

			items, err := meta.ExtractList(list)
			if err != nil {
				log.Fatal(err)
			}

			for _, item := range items {
				unstructured, ok := item.(runtime.Unstructured)
				if !ok {
					panic("assertion failed")
				}

				metadata, err := meta.Accessor(unstructured)
				if err != nil {
					log.Fatal(err)
				}

				resourceList := Metrics{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &resourceList); err != nil {
					log.Fatalf("conversion failed: %s", err)
				}

				metrics.SetNodeResourceUsage(metadata.GetName(), "cpu", float64(resourceList.Usage.Cpu().MilliValue()))
				metrics.SetNodeResourceUsage(metadata.GetName(), "memory", float64(resourceList.Usage.Memory().ScaledValue(resource.Kilo)))
			}
		}
	}

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	http.ListenAndServe(":8080", nil)
}
