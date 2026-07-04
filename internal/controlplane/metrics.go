package controlplane

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentctl_cp_http_requests_total",
		Help: "HTTP requests handled by the control plane, by route and status.",
	}, []string{"route", "method", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agentctl_cp_http_request_duration_seconds",
		Help:    "HTTP request latency by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})

	licenseActivationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentctl_cp_license_activations_total",
		Help: "Successful license activations/refreshes, by plan.",
	}, []string{"plan"})

	webhookEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentctl_cp_webhook_events_total",
		Help: "Freemius webhook deliveries processed, by event type and outcome.",
	}, []string{"event", "outcome"})
)

// instrumentRoute wraps a handler so every request updates the request-count
// and latency metrics under a fixed route label (never the raw URL path, to
// keep cardinality bounded).
func instrumentRoute(route string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h(rec, r)
		httpRequestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		httpRequestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func metricsHandler() http.Handler {
	return promhttp.Handler()
}
