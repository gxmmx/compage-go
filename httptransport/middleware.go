package httptransport

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	apperrors "github.com/gxmmx/compage-go/errors"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	err        error
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) writeError(err error) {
	lrw.err = err
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

		// log unsuccessful request
		if lrw.err != nil {
			var appErr *apperrors.AppError
			if errors.As(lrw.err, &appErr) {
				if appErr.Kind == apperrors.KindInternal {
					c.log.ErrorContext(r.Context(), "request error", "class", "event", "ip", clientIP, "method", r.Method, "path", r.URL.Path, "status", status, "duration", duration, "error", appErr.Error())
					return
				} else {
					c.log.WarnContext(r.Context(), "request unsuccessful", "class", "event", "ip", clientIP, "method", r.Method, "path", r.URL.Path, "status", status, "duration", duration)
					return
				}
			} else {
				c.log.ErrorContext(r.Context(), "request error", "class", "event", "ip", clientIP, "method", r.Method, "path", r.URL.Path, "status", status, "duration", duration, "error", lrw.err.Error())
			}
		}
		// log successful request
		c.log.InfoContext(r.Context(), "request successful", "class", "event", "ip", clientIP, "method", r.Method, "path", r.URL.Path, "status", status, "duration", duration)
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
