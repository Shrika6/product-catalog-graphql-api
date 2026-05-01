package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpInflightRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_inflight_requests",
		Help: "Current number of in-flight HTTP requests.",
	})

	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests by method, path, and status.",
	}, []string{"method", "path", "status"})

	httpRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds by method, path, and status.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"method", "path", "status"})

	graphqlOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_operations_total",
		Help: "Total GraphQL resolver/operation invocations.",
	}, []string{"operation"})

	graphqlErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_errors_total",
		Help: "Total GraphQL resolver/operation errors.",
	}, []string{"operation"})

	graphqlResolverDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "graphql_resolver_duration_seconds",
		Help:    "GraphQL resolver latency in seconds by operation.",
		Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
	}, []string{"operation"})

	dbQueryDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Database query duration in seconds by repository method.",
		Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
	}, []string{"repository", "method"})

	cacheOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_operations_total",
		Help: "Total cache operations by cache, operation, and result.",
	}, []string{"cache", "operation", "result"})

	cacheOperationDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_operation_duration_seconds",
		Help:    "Cache operation duration in seconds by cache, operation, and result.",
		Buckets: []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05},
	}, []string{"cache", "operation", "result"})

	cacheMissPenaltySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_miss_penalty_seconds",
		Help:    "Latency penalty in seconds when cache miss falls back to source of truth.",
		Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
	}, []string{"cache", "operation"})
)

func HTTPRequestStarted() {
	httpInflightRequests.Inc()
}

func HTTPRequestFinished(method, path string, statusCode int, duration time.Duration) {
	httpInflightRequests.Dec()
	status := strconv.Itoa(statusCode)
	httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, path, status).Observe(duration.Seconds())
}

func GraphQLOperationStarted(operation string) {
	graphqlOperationsTotal.WithLabelValues(operation).Inc()
}

func GraphQLOperationFinished(operation string, duration time.Duration, err error) {
	graphqlResolverDurationSeconds.WithLabelValues(operation).Observe(duration.Seconds())
	if err != nil {
		graphqlErrorsTotal.WithLabelValues(operation).Inc()
	}
}

func DBQueryFinished(repository, method string, duration time.Duration) {
	dbQueryDurationSeconds.WithLabelValues(repository, method).Observe(duration.Seconds())
}

func CacheOperationFinished(cache, operation, result string, duration time.Duration) {
	cacheOperationsTotal.WithLabelValues(cache, operation, result).Inc()
	cacheOperationDurationSeconds.WithLabelValues(cache, operation, result).Observe(duration.Seconds())
}

func CacheMissPenaltyObserved(cache, operation string, duration time.Duration) {
	cacheMissPenaltySeconds.WithLabelValues(cache, operation).Observe(duration.Seconds())
}
