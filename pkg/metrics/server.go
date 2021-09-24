package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var EnvoyAdsRegistry = prometheus.NewRegistry()
var AtlasRegistry = prometheus.NewRegistry()

// NewMetricsServer --
func NewMetricsServer(ctx context.Context, port string, debug bool, registry *prometheus.Registry) error {
	if registry != nil {
		registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
		registry.MustRegister(prometheus.NewGoCollector())
		registry.MustRegister(prometheus.NewBuildInfoCollector())
	}

	log := logrus.WithField("component", "metrics").WithField("port", port)

	router := mux.NewRouter().StrictSlash(true)

	if registry == nil {
		router.Path("/metrics").Handler(promhttp.Handler())
	} else {
		handler := promhttp.InstrumentMetricHandler(registry, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			Registry: registry,
		}))

		router.Path("/metrics").Handler(handler)
	}

	if debug {
		// Register pprof handlers
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
		router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	go func() {
		log.Info("Starting Metrics Server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("an error occurred with metrics server")
		}
	}()

	<-ctx.Done()

	log.Info("Shutting down metrics server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv.SetKeepAlivesEnabled(false)
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Could not gracefully shutdown the metrics server: %v\n", err)
	}

	return nil
}
