package metrics

import (
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (m *Metrics) InstrumentHTTP(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rec, r)

		m.HTTPServerRequestDuration.WithLabelValues(
			httpRouteLabel(r),
			r.Method,
			HTTPStatusClass(rec.statusCode),
		).Observe(time.Since(start).Seconds())
	})
}

func HTTPStatusClass(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "1xx"
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

func httpRouteLabel(r *http.Request) string {
	if pattern := strings.TrimSpace(r.Pattern); pattern != "" {
		if _, route, ok := strings.Cut(pattern, " "); ok {
			return route
		}
		return pattern
	}

	switch path := r.URL.Path; {
	case path == "/metrics":
		return "/metrics"
	case strings.HasPrefix(path, "/api/auth/"):
		return "/api/auth/*"
	case strings.HasPrefix(path, "/api/triggers/http/"):
		return "/api/triggers/http/"
	case strings.HasPrefix(path, "/api/admin/"):
		return "/api/admin/*"
	case strings.HasPrefix(path, "/oauth/"):
		return "/oauth/*"
	case strings.HasPrefix(path, "/admin/"):
		return "/admin/*"
	case strings.HasPrefix(path, "/api/"):
		return "/api/*"
	case path == "/":
		return "/"
	default:
		return "unmatched"
	}
}
