package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// MetricsRegistry keeps process-local HTTP counters for Prometheus scraping.
type MetricsRegistry struct {
	mu        sync.RWMutex
	startedAt time.Time
	requests  map[string]int64
	errors    map[string]int64
	buckets   map[string][]int64
	sum       map[string]float64
}

// NewMetricsRegistry creates an empty metrics registry.
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		startedAt: time.Now(),
		requests:  map[string]int64{},
		errors:    map[string]int64{},
		buckets:   map[string][]int64{},
		sum:       map[string]float64{},
	}
}

var defaultMetrics = NewMetricsRegistry()

// MetricsMiddleware records request count, error count, and duration histograms.
func MetricsMiddleware(next http.Handler) http.Handler {
	return defaultMetrics.Middleware(next)
}

// DefaultMetrics returns the process-wide metrics registry.
func DefaultMetrics() *MetricsRegistry {
	return defaultMetrics
}

// Middleware records metrics for an HTTP handler.
func (m *MetricsRegistry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		m.Observe(r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

// Observe records one HTTP request.
func (m *MetricsRegistry) Observe(method, path string, status int, duration time.Duration) {
	route := normalizeRoute(path)
	statusClass := fmt.Sprintf("%dxx", status/100)
	key := metricKey(method, route, statusClass)
	seconds := duration.Seconds()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[key]++
	if status >= 500 {
		m.errors[metricKey(method, route, "")]++
	}
	if _, ok := m.buckets[key]; !ok {
		m.buckets[key] = make([]int64, len(durationBuckets)+1)
	}
	for i, bucket := range durationBuckets {
		if seconds <= bucket {
			m.buckets[key][i]++
		}
	}
	m.buckets[key][len(durationBuckets)]++
	m.sum[key] += seconds
}

// WritePrometheus writes the registry in Prometheus text exposition format.
func (m *MetricsRegistry) WritePrometheus(w http.ResponseWriter) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, _ = fmt.Fprintf(w, "# HELP skillforge_process_uptime_seconds Seconds since this process started.\n# TYPE skillforge_process_uptime_seconds gauge\nskillforge_process_uptime_seconds %.0f\n", time.Since(m.startedAt).Seconds())

	_, _ = fmt.Fprint(w, "# HELP skillforge_http_requests_total Total HTTP requests by method, route, and status class.\n# TYPE skillforge_http_requests_total counter\n")
	for _, key := range sortedKeys(m.requests) {
		method, route, statusClass := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "skillforge_http_requests_total{method=%q,route=%q,status_class=%q} %d\n", method, route, statusClass, m.requests[key])
	}

	_, _ = fmt.Fprint(w, "# HELP skillforge_http_errors_total Total 5xx HTTP responses by method and route.\n# TYPE skillforge_http_errors_total counter\n")
	for _, key := range sortedKeys(m.errors) {
		method, route, _ := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "skillforge_http_errors_total{method=%q,route=%q} %d\n", method, route, m.errors[key])
	}

	_, _ = fmt.Fprint(w, "# HELP skillforge_http_request_duration_seconds HTTP request duration histogram.\n# TYPE skillforge_http_request_duration_seconds histogram\n")
	for _, key := range sortedKeys(m.buckets) {
		method, route, statusClass := splitMetricKey(key)
		for i, bucket := range durationBuckets {
			_, _ = fmt.Fprintf(w, "skillforge_http_request_duration_seconds_bucket{method=%q,route=%q,status_class=%q,le=%q} %d\n", method, route, statusClass, fmt.Sprintf("%g", bucket), m.buckets[key][i])
		}
		_, _ = fmt.Fprintf(w, "skillforge_http_request_duration_seconds_bucket{method=%q,route=%q,status_class=%q,le=%q} %d\n", method, route, statusClass, "+Inf", m.buckets[key][len(durationBuckets)])
		_, _ = fmt.Fprintf(w, "skillforge_http_request_duration_seconds_sum{method=%q,route=%q,status_class=%q} %g\n", method, route, statusClass, m.sum[key])
		_, _ = fmt.Fprintf(w, "skillforge_http_request_duration_seconds_count{method=%q,route=%q,status_class=%q} %d\n", method, route, statusClass, m.buckets[key][len(durationBuckets)])
	}
}

func metricKey(method, route, statusClass string) string {
	return method + "\x00" + route + "\x00" + statusClass
}

func splitMetricKey(key string) (string, string, string) {
	parts := strings.SplitN(key, "\x00", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeRoute(path string) string {
	if path == "" {
		return "/"
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if looksDynamicSegment(part) {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

func looksDynamicSegment(part string) bool {
	if len(part) >= 8 {
		hasDigit := false
		hasHexish := true
		for _, r := range part {
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '-') {
				hasHexish = false
			}
		}
		if hasDigit && hasHexish {
			return true
		}
	}
	for _, r := range part {
		if r < '0' || r > '9' {
			return false
		}
	}
	return part != ""
}
