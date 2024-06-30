package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ServerRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nntplexer",
		Subsystem: "server",
		Name:      "requests_total",
		Help:      "Number of requests by command",
	}, []string{"command"})

	ServerResponses = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nntplexer",
		Subsystem: "server",
		Name:      "responses_total",
		Help:      "Number of responses by code",
	}, []string{"code"})

	ServerSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nntplexer",
		Subsystem: "server",
		Name:      "sessions_total",
		Help:      "Number of sessions",
	})

	ArticleRequests = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nntplexer",
		Subsystem: "nntp",
		Name:      "article_requests_total",
		Help:      "Number of aricles requested",
	})

	BackendRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nntplexer",
		Subsystem: "nntp",
		Name:      "backend_requests_total",
		Help:      "Number of aricles fetched by backend and response code",
	}, []string{"backend", "code"})

	BackendBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nntplexer",
		Subsystem: "nntp",
		Name:      "backend_bytes",
		Help:      "Number of bytes served by backend",
	}, []string{"backend"})
)
