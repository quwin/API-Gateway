package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "api_gateway",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests received by the gateway.",
		},
		[]string{"method", "path", "status"},
	)

	httpRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "api_gateway",
			Name:      "rate_limit_rejections_total",
			Help:      "Total number of requests rejected by the rate limiter.",
		},
		[]string{"method", "path", "api_key_present"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "api_gateway",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets: []float64{
				0.001,
				0.005,
				0.01,
				0.025,
				0.05,
				0.1,
				0.25,
				0.5,
				1,
				2.5,
				5,
			},
		},
		[]string{"method", "path", "status"},
	)

	httpInFlightRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "api_gateway",
			Name:      "http_in_flight_requests",
			Help:      "Current number of in-flight HTTP requests.",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRejectedTotal)
	prometheus.MustRegister(httpRequestDurationSeconds)
	prometheus.MustRegister(httpInFlightRequests)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := newStatusRecorder(w)

		httpInFlightRequests.Inc()
		defer httpInFlightRequests.Dec()

		next.ServeHTTP(recorder, r)

		status := strconv.Itoa(recorder.status)
		path := routeLabel(r)
		method := r.Method

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())

		if recorder.status == http.StatusTooManyRequests {
			apiKeyPresent := "false"
			if r.Header.Get("X-API-Key") != "" {
				apiKeyPresent = "true"
			}

			httpRejectedTotal.WithLabelValues(method, path, apiKeyPresent).Inc()
		}
	})
}

func routeLabel(r *http.Request) string {
	// Uses the URL path because routes are simple.
	// For production, I should avoid high-cardinality labels like raw user IDs or arbitrary paths.
	if r.URL.Path == "" {
		return "/"
	}
	return r.URL.Path
}