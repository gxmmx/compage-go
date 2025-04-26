package httptransport

import "net/http"

func getClientIP(r *http.Request) string {
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded // Use the first value in X-Forwarded-For if it exists
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}
	return clientIP
}

func calculateNextPage(currentPage, perPage, totalCount int) int {
	if currentPage*perPage >= totalCount {
		return 0
	}
	return currentPage + 1
}
