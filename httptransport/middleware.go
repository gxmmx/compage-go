package httptransport

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	uuid "github.com/google/uuid"
	apperrors "github.com/gxmmx/compage-go/errors"
)

// -----------------------------------------------------------------------------
// Logging middleware
// -----------------------------------------------------------------------------

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

// Middleware to add logging to the request
func (c *Controller) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		clientIP := getClientIP(r)

		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)
		duration := time.Since(start).String()
		status := strconv.Itoa(lrw.statusCode)

		// log unsuccessful request
		logFields := []interface{}{
			"class", "event",
			"ip", clientIP,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration", duration,
		}

		requestID := getRequestIdFromRequest(r)
		if requestID != "" {
			logFields = append(logFields, "requestID", requestID)
		}

		if lrw.err != nil {
			var appErr *apperrors.AppError
			if errors.As(lrw.err, &appErr) {
				if appErr.Kind == apperrors.KindInternal {
					c.log.ErrorContext(r.Context(), "request error", append(logFields, "error", appErr.Error())...)
				} else {
					c.log.WarnContext(r.Context(), "request unsuccessful", append(logFields, "error", appErr.Error())...)
				}
			} else {
				c.log.ErrorContext(r.Context(), "request error", append(logFields, "error", lrw.err.Error())...)
			}
		} else {
			// log successful request
			c.log.InfoContext(r.Context(), "request successful", logFields...)
		}
	})
}

// -----------------------------------------------------------------------------
// Request ID middleware
// -----------------------------------------------------------------------------

type requestIDKey string

const rIdKey requestIDKey = "requestID"

// Middleware to handle request ID
func (c *Controller) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Try to get Request ID from header
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// Generate a new one if not provided
			requestID = uuid.New().String()
			w.Header().Set("X-Request-ID", requestID)
		}

		// Add it into the context
		ctx = context.WithValue(ctx, rIdKey, requestID)

		// Pass to next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
