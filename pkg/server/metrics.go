package server

import (
	"github.com/prometheus/client_golang/prometheus"
)

type HttpMetrics struct {
	handledTotalCount *prometheus.CounterVec
	handledHistogram  *prometheus.HistogramVec
}

func NewHttpMetrics(reg prometheus.Registerer) *HttpMetrics {
	s := &HttpMetrics{
		handledTotalCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of requests received on the server.",
			}, []string{"handler", "code", "method", "path"}),
		handledHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Histogram of response latency (seconds) of that had been application-level handled by the server.",
				Buckets: prometheus.DefBuckets,
			}, []string{"handler", "code", "method", "path"}),
	}
	reg.MustRegister(s.handledTotalCount, s.handledHistogram)
	return s
}
