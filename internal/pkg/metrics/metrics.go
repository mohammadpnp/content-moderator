package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics (RED)
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	// Business Metrics
	ContentsCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "contents_created_total",
			Help: "Total number of content items created",
		},
		[]string{"type", "status"},
	)

	ModerationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "moderation_duration_seconds",
			Help:    "Duration of AI moderation in seconds",
			Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"model_name"},
	)

	ModerationResultsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "moderation_results_total",
			Help: "Total number of moderation results",
		},
		[]string{"is_approved", "model_name"},
	)

	ActiveWsConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_ws_connections",
			Help: "Current number of active WebSocket connections",
		},
	)

	JobQueueLength = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_queue_length",
			Help: "Current number of pending moderation jobs in queue",
		},
	)
)
