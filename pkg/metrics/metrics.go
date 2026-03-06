package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartMetricsServer starts an HTTP server for Prometheus scraping.
func StartMetricsServer(addr string) error {
	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, nil)
}
