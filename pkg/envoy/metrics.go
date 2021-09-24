package envoy

import (
	"github.com/ekristen/atlas/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	connectedClients = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "atlas_envoy_ads_connected_clients",
		Help: "The total number of connected Envoy Proxies",
	})
	snapshots = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "atlas_envoy_ads_snapshots_total",
		Help: "The number of snapshots generated for Envoy ADS server",
	})
)

func init() {
	metrics.EnvoyAdsRegistry.MustRegister(connectedClients)
	metrics.EnvoyAdsRegistry.MustRegister(snapshots)
}
