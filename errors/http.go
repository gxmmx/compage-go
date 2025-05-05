package errors

import "net/http"

func (e *AppError) HTTPStatus() int {
	switch e.Kind {
	case KindAlreadyExists, KindConflict:
		return http.StatusConflict
	case KindNotFound:
		return http.StatusNotFound
	case KindInvalidInput:
		return http.StatusBadRequest
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindTimeout, KindUnavailable:
		return http.StatusGatewayTimeout
	case KindMethodNotAllowed:
		return http.StatusMethodNotAllowed
	default:
		return http.StatusInternalServerError
	}
}
