This project is archived and no longer maintained. Metrics Server was never meant to be used for non-autoscaling purposes and this
project was more like a proof of concept back in the time I was working with k8s < 1.18. Since k8s 1.18+ kubelets expose more
accurate resource usage metrics on `/metrics/resource` endpoint, you can scrape this endpoint instead.

![ci](https://github.com/ghouscht/metrics-server-exporter/workflows/ci/badge.svg)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/ghouscht/metrics-server-exporter)
[![Go Report Card](https://goreportcard.com/badge/github.com/ghouscht/metrics-server-exporter)](https://goreportcard.com/report/github.com/ghouscht/metrics-server-exporter)
![License](https://img.shields.io/github/license/ghouscht/metrics-server-exporter)

# metrics-server-exporter
Export [metrics-server](https://github.com/kubernetes-sigs/metrics-server) metrics to prometheus.

*Metrics Server collects resource metrics from Kubelets and exposes them in Kubernetes apiserver through Metrics API for use by Horizontal Pod Autoscaler and Vertical Pod Autoscaler.*

But why not use these metrics to create alerts with [alertmanager](https://github.com/prometheus/alertmanager) or to visualize resource usage with [grafana](https://github.com/grafana/grafana)? Metrics-server-exporter aims to fill the gap between monitoring/alerting tools and metrics-server by exporting the data from the Metrics API to prometheus.

### Exported metrics
```
metrics_server_exporter_node_resource_capacity{node="node1",resource="cpu"} 42000    # millicores
metrics_server_exporter_node_resource_capacity{node="node1",resource="memory"} 42000 # kilobyte
metrics_server_exporter_node_resource_usage{node="node1",resource="cpu"} 1000        # millicores
metrics_server_exporter_node_resource_usage{node="node1",resource="memory"} 1000     # kilobyte
metrics_server_exporter_pod_resource_usage{namespace="kube-system",pod="metrics-server-exporter-68f7c886bf-wj8h6",resource="cpu"} 42 # millicores
metrics_server_exporter_pod_resource_usage{namespace="kube-system",pod="metrics-server-exporter-68f7c886bf-wj8h6",resource="memory"} 42 # kilobyte

```

## Deployment

### prerequisites
* [metrics-server](https://github.com/kubernetes-sigs/metrics-server) must be deployed and fully operational in your cluster.
* [prometheus](https://github.com/prometheus/prometheus) to scrape the metrics-server-exporter metrics

### installation
Simply apply the [deployment.yaml](https://github.com/ghouscht/metrics-server-exporter/blob/master/deployment.yaml) from this repo. This will create a ClusterRole, a ClusterRoleBinding, a ServiceAccount and the Deployment for the metrics-server-exporter.
