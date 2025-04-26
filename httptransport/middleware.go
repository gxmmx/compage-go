package httptransport

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Adds logging to the request
func (c *Controller) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		clientIP := getClientIP(r)

		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)
		duration := time.Since(start).String()
		status := strconv.Itoa(lrw.statusCode)
		c.log.InfoContext(r.Context(), "admin api request", "class", "event", "ip", clientIP, "method", r.Method, "path", r.URL.Path, "status", status, "duration", duration)
	})
}

// Injects a context into the request
func (c *Controller) contextMiddleware(ctx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
