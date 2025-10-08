// ============================================
// 2. METRICS (Prometheus)
// ============================================
package utils

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeRequests  *prometheus.GaugeVec
	errorCounter    *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "unicorn_requests_total",
				Help: "Total number of requests",
			},
			[]string{"service", "method", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "unicorn_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "method"},
		),
		activeRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "unicorn_active_requests",
				Help: "Number of active requests",
			},
			[]string{"service"},
		),
		errorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "unicorn_errors_total",
				Help: "Total number of errors",
			},
			[]string{"service", "error_type"},
		),
	}

	// Register metrics
	prometheus.MustRegister(m.requestCounter)
	prometheus.MustRegister(m.requestDuration)
	prometheus.MustRegister(m.activeRequests)
	prometheus.MustRegister(m.errorCounter)

	return m
}

func (m *Metrics) RecordRequest(service, method string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
		m.errorCounter.WithLabelValues(service, "request_error").Inc()
	}

	m.requestCounter.WithLabelValues(service, method, status).Inc()
	m.requestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

func (m *Metrics) IncrementActiveRequests(service string) {
	m.activeRequests.WithLabelValues(service).Inc()
}

func (m *Metrics) DecrementActiveRequests(service string) {
	m.activeRequests.WithLabelValues(service).Dec()
}

func (m *Metrics) GetHandler() http.Handler {
	return promhttp.Handler()
}
