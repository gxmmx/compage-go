package httptransport

import (
	"net/http"

	// Compage
	apperrors "github.com/gxmmx/compage-go/errors"

	// Third party
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Retrieves the client's IP address from the request
func getClientIP(r *http.Request) string {
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded // Use the first value in X-Forwarded-For if it exists
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}
	return clientIP
}

// Calculates te next page number based on the current page, items per page, and total count
func calculateNextPage(currentPage, perPage, totalCount int) int {
	if currentPage*perPage >= totalCount {
		return 0
	}
	return currentPage + 1
}

// notFoundHandler returns a 404 Not Found response
func notFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := NewResponse(w, r, false)
		response.Error(apperrors.NotFound(nil, "page not found"))
	})
}

// Turns a slice of specific types into a slice of any for printing
func ToAnySlice[T any](input []T) []any {
	out := make([]any, len(input))
	for i, v := range input {
		out[i] = v
	}
	return out
}

// Gets, Validates and returns a uuid
func GetUUID(r *http.Request, key string) (uuid.UUID, error) {
	id := mux.Vars(r)[key]
	if id == "" {
		return uuid.Nil, apperrors.InvalidInput(nil, "missing id")
	}
	validUuid, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, apperrors.InvalidInput(nil, "invalid id").WithField("id", id)
	}
	return validUuid, nil
}

// Gets the request ID from the context
func getRequestIdFromRequest(r *http.Request) string {
	if requestID, ok := r.Context().Value(rIdKey).(string); ok {
		return requestID
	}
	return ""
}
